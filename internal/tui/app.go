package tui

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/itsolver/zentui/internal/browser"
	"github.com/itsolver/zentui/internal/codexrunner"
	"github.com/itsolver/zentui/internal/permissions"
	"github.com/itsolver/zentui/internal/triage"
	"github.com/itsolver/zentui/internal/types"
	"github.com/itsolver/zentui/pkg/zendesk"
)

type errMsg struct{ err error }

type cursorSettledMsg struct {
	seq uint64
	id  int64
}

type viewState int

const (
	listView viewState = iota
	detailView
	splitView
	kanbanView
)

type panelFocus int

const (
	focusList panelFocus = iota
	focusDetail
	focusOperator
)

type currentUserMsg struct{ user *types.User }

type draftGeneratedMsg struct {
	ticketID int64
	output   triage.DraftOutput
}

type draftErrMsg struct{ err error }

type assetsPreparedMsg struct {
	ticketID int64
	manifest triage.Manifest
	analysis map[string]triage.ImageAnalysis
	err      error
}

type mergePreparedMsg struct {
	ticketID            int64
	suggestions         []triage.MergeSuggestion
	recommendedTargetID int64
}

type mergePrepareErrMsg struct{ err error }

type inlineFieldUpdatedMsg struct {
	ticketID int64
	ticket   *types.Ticket
}

type inlineFieldErrMsg struct{ err error }

type App struct {
	tickets     zendesk.TicketService
	search      zendesk.SearchService
	users       zendesk.UserService
	subdomain   string
	currentUser *types.User
	perms       permissions.Permissions
	state       viewState
	prevState   viewState // saved state when entering detail from kanban
	list        listModel
	detail      detailModel
	kanban      kanbanModel
	actions     actionsModel
	operator    operatorModel
	searchM     searchModel
	gotoM       gotoModel
	cmdPalette  cmdPaletteModel
	codex       codexrunner.Runner
	workCache   triage.WorkCache
	pythonBin   string
	draftBusy   bool
	draftErr    error
	mergeBusy   bool
	mergeErr    error
	width       int
	height      int
	focus       panelFocus
	showDetail  bool
	version     string
	cursorSeq   uint64
	promptEnv   []string
	openPath    func(string)
	notice      string

	mouseClickPending bool
}

type AppOptions struct {
	ViewID              int64
	Limit               int
	CustomerSupportDir  string
	CodexModel          string
	CodexReasoning      string
	PythonBin           string
	WorkDir             string
	PromptPackEnv       []string
	HTTPClient          *http.Client
	UntrustedHTTPClient *http.Client
	TrustedHosts        []string
}

func NewApp(tickets zendesk.TicketService, search zendesk.SearchService, users zendesk.UserService, subdomain, version string) App {
	return NewAppWithOptions(tickets, search, users, subdomain, version, AppOptions{})
}

func NewAppWithOptions(tickets zendesk.TicketService, search zendesk.SearchService, users zendesk.UserService, subdomain, version string, opts AppOptions) App {
	codex := codexrunner.Runner{
		CustomerSupportDir: opts.CustomerSupportDir,
		Model:              opts.CodexModel,
		ReasoningEffort:    opts.CodexReasoning,
	}
	return App{
		tickets:    tickets,
		search:     search,
		users:      users,
		subdomain:  subdomain,
		version:    version,
		perms:      permissions.FromUser(nil),
		state:      listView,
		showDetail: false,
		focus:      focusList,
		list:       newListModelWithOptions(tickets, search, opts.ViewID, opts.Limit),
		detail:     newDetailModel(tickets),
		kanban:     newKanbanModel(),
		actions:    newActionsModel(tickets, users),
		operator:   newOperatorModel(),
		searchM:    newSearchModel(),
		gotoM:      newGotoModel(),
		cmdPalette: newCmdPaletteModel(),
		codex:      codex,
		workCache:  triage.WorkCache{Root: opts.WorkDir, HTTPClient: opts.HTTPClient, UntrustedHTTPClient: opts.UntrustedHTTPClient, TrustedHosts: opts.TrustedHosts},
		pythonBin:  opts.PythonBin,
		promptEnv:  append([]string(nil), opts.PromptPackEnv...),
		openPath:   browser.Open,
	}
}

func (m App) Init() tea.Cmd {
	return tea.Batch(m.list.Init(), m.fetchCurrentUser(), m.fetchTicketFields(), operatorTick())
}

func (m App) fetchCurrentUser() tea.Cmd {
	return func() tea.Msg {
		if m.users == nil {
			return currentUserMsg{}
		}
		user, err := m.users.GetMe(context.Background())
		if err != nil {
			return currentUserMsg{}
		}
		return currentUserMsg{user: user}
	}
}

func (m App) fetchTicketFields() tea.Cmd {
	return func() tea.Msg {
		if m.tickets == nil {
			return ticketFieldsLoadedMsg{}
		}
		page, err := m.tickets.ListTicketFields(context.Background(), &types.ListTicketFieldsOptions{Limit: 100})
		if err != nil {
			return ticketFieldsLoadedMsg{}
		}
		return ticketFieldsLoadedMsg{fields: page.TicketFields}
	}
}

func (m App) listPanelWidth() int {
	if m.state != splitView || !m.showDetail {
		return m.width
	}
	if m.operatorPanelWidth() > 0 {
		w := (m.width - m.operatorPanelWidth() - 2) * 34 / 100
		if w < 34 {
			w = 34
		}
		return w
	}
	return (m.width - 1) / 2 // -1 for divider
}

func (m App) detailPanelWidth() int {
	if opW := m.operatorPanelWidth(); opW > 0 {
		return m.width - m.listPanelWidth() - opW - 2 // two dividers
	}
	return m.width - m.listPanelWidth() - 1 // -1 for divider
}

func (m App) operatorPanelWidth() int {
	if m.state != splitView || !m.showDetail || m.width < 140 {
		return 0
	}
	w := m.width / 5
	if w < 30 {
		w = 30
	}
	if w > 42 {
		w = 42
	}
	return w
}

func (m App) autoLoadFirstTicket() tea.Cmd {
	if m.state != splitView || !m.showDetail {
		return nil
	}
	if len(m.list.items) == 0 {
		return nil
	}
	id := m.list.items[m.list.cursor].ID
	m.detail = newDetailModel(m.tickets)
	m.detail.expectedID = id
	w := m.detailPanelWidth()
	m.detail.width = w
	m.detail.height = m.height
	return tea.Batch(m.detail.spinner.Tick, m.detail.loadTicket(id), m.detail.loadAudits(id))
}

func (m *App) reloadDetailIfVisible() []tea.Cmd {
	if m.state != splitView || !m.showDetail {
		return nil
	}
	if len(m.list.items) > 0 {
		id := m.list.items[m.list.cursor].ID
		m.detail = newDetailModel(m.tickets)
		m.detail.expectedID = id
		m.detail.width = m.detailPanelWidth()
		m.detail.height = m.height
		return []tea.Cmd{m.detail.spinner.Tick, m.detail.loadTicket(id), m.detail.loadAudits(id)}
	}
	// Clear detail panel when no items
	m.detail = newDetailModel(m.tickets)
	m.detail.loading = false
	m.detail.width = m.detailPanelWidth()
	m.detail.height = m.height
	return nil
}

func (m *App) loadDetailForCursor() tea.Cmd {
	if len(m.list.items) == 0 {
		return nil
	}
	id := m.list.items[m.list.cursor].ID
	// Don't reload if already showing this ticket
	if m.detail.ticket != nil && m.detail.ticket.ID == id {
		return nil
	}
	m.detail = newDetailModel(m.tickets)
	m.detail.expectedID = id
	w := m.detailPanelWidth()
	m.detail.width = w
	m.detail.height = m.height
	return tea.Batch(m.detail.spinner.Tick, m.detail.loadTicket(id), m.detail.loadAudits(id))
}

func (m App) windowTitle() string {
	switch m.state {
	case detailView:
		if m.detail.ticket != nil {
			subject := m.detail.ticket.Subject
			if len([]rune(subject)) > 50 {
				subject = string([]rune(subject)[:50]) + "…"
			}
			return fmt.Sprintf("zentui — #%d: %s", m.detail.ticket.ID, subject)
		}
		return "zentui — Loading..."
	case listView, splitView, kanbanView:
		if m.list.loading {
			return "zentui — Loading..."
		}
		if len(m.list.items) == 0 {
			return "zentui — No tickets"
		}
		if m.list.searchQuery != "" {
			q := m.list.searchQuery
			if len([]rune(q)) > 40 {
				q = string([]rune(q)[:40]) + "…"
			}
			return fmt.Sprintf("zentui — Search: %q (%d results)", q, len(m.list.items))
		}
		newCount := len(m.list.newTicketIDs)
		if newCount > 0 {
			return fmt.Sprintf("zentui — %d tickets (%d new)", len(m.list.items), newCount)
		}
		return fmt.Sprintf("zentui — %d tickets", len(m.list.items))
	}
	return "zentui"
}

func (m App) updateWindowTitle() tea.Cmd {
	return nil
}

func ringBell() tea.Cmd {
	return func() tea.Msg {
		os.Stderr.Write([]byte("\a"))
		return nil
	}
}

func (m App) generateDraft(ticket types.Ticket) tea.Cmd {
	codex := m.codex
	cache := m.workCache
	pythonBin := m.pythonBin
	promptEnv := append([]string(nil), m.promptEnv...)
	return func() tea.Msg {
		analysis, _ := cache.ReadImageAnalysis(ticket.ID)
		pack, err := triage.BuildDraftPromptPack(context.Background(), codex.CustomerSupportDir, pythonBin, ticket.ID, "public", analysis, promptEnv...)
		if err != nil {
			return draftErrMsg{err: err}
		}
		result, err := codex.RunPrompt(context.Background(), pack.Prompt, pack.Schema, nil)
		if err != nil {
			return draftErrMsg{err: err}
		}
		_ = cache.AppendCodexRun(ticket.ID, map[string]any{
			"kind":       "draft",
			"ticket_id":  ticket.ID,
			"usage":      result.Usage,
			"created_at": time.Now().UTC().Format(time.RFC3339),
		})
		output, err := codexrunner.DecodeOutput[triage.DraftOutput](result.Output)
		if err != nil {
			return draftErrMsg{err: err}
		}
		normalized, err := triage.NormalizeDraftPromptPackResult(context.Background(), codex.CustomerSupportDir, pythonBin, pack.Mode, output, promptEnv...)
		if err != nil {
			return draftErrMsg{err: err}
		}
		return draftGeneratedMsg{ticketID: ticket.ID, output: normalized}
	}
}

func (m App) prepareAssets(ticketID int64, audits []types.Audit) tea.Cmd {
	cache := m.workCache
	codex := m.codex
	pythonBin := m.pythonBin
	promptEnv := append([]string(nil), m.promptEnv...)
	return func() tea.Msg {
		if ticketID == 0 {
			return nil
		}
		if _, err := cache.EnsureTicketDir(ticketID); err != nil {
			return assetsPreparedMsg{ticketID: ticketID, err: err}
		}
		sources := triage.ExtractImageSourcesFromAudits(audits)
		for _, source := range sources {
			if _, err := cache.DownloadImage(context.Background(), ticketID, source); err != nil {
				return assetsPreparedMsg{ticketID: ticketID, err: err}
			}
		}
		manifest, err := cache.ReadManifest(ticketID)
		if err != nil {
			return assetsPreparedMsg{ticketID: ticketID, err: err}
		}
		analysis, err := cache.ReadImageAnalysis(ticketID)
		if err != nil {
			return assetsPreparedMsg{ticketID: ticketID, err: err}
		}
		for _, asset := range manifest.Assets {
			if asset.Skipped || asset.SHA256 == "" || analysis[asset.SHA256].Summary != "" {
				continue
			}
			pack, err := triage.BuildImagePromptPack(context.Background(), codex.CustomerSupportDir, pythonBin, ticketID, asset.Filename, asset.SourceURL, "", promptEnv...)
			if err != nil {
				return assetsPreparedMsg{ticketID: ticketID, manifest: manifest, analysis: analysis, err: err}
			}
			result, err := codex.RunPrompt(context.Background(), pack.Prompt, pack.Schema, []string{asset.LocalPath})
			if err != nil {
				return assetsPreparedMsg{ticketID: ticketID, manifest: manifest, analysis: analysis, err: err}
			}
			output, err := codexrunner.DecodeOutput[triage.ImageOutput](result.Output)
			if err != nil {
				return assetsPreparedMsg{ticketID: ticketID, manifest: manifest, analysis: analysis, err: err}
			}
			analysis[asset.SHA256] = triage.ImageAnalysis{
				Summary:           output.Summary,
				VisibleText:       output.VisibleText,
				IsSignatureOrLogo: output.IsSignatureOrLogo,
				Relevance:         output.Relevance,
			}
			_ = cache.AppendCodexRun(ticketID, map[string]any{
				"kind":       "image",
				"ticket_id":  ticketID,
				"asset":      asset.Filename,
				"sha256":     asset.SHA256,
				"usage":      result.Usage,
				"created_at": time.Now().UTC().Format(time.RFC3339),
			})
		}
		if err := cache.WriteImageAnalysis(ticketID, analysis); err != nil {
			return assetsPreparedMsg{ticketID: ticketID, manifest: manifest, analysis: analysis, err: err}
		}
		return assetsPreparedMsg{ticketID: ticketID, manifest: manifest, analysis: analysis}
	}
}

func (m App) generateMergeSuggestions(ticket types.Ticket) tea.Cmd {
	codex := m.codex
	pythonBin := m.pythonBin
	promptEnv := append([]string(nil), m.promptEnv...)
	return func() tea.Msg {
		pool, err := triage.BuildMergePool(context.Background(), codex.CustomerSupportDir, pythonBin, ticket.ID, promptEnv...)
		if err != nil {
			return mergePrepareErrMsg{err: err}
		}
		if pool.Status != "success" || len(pool.Candidates) == 0 {
			return mergePreparedMsg{ticketID: ticket.ID}
		}
		pack, err := triage.BuildMergePromptPack(context.Background(), codex.CustomerSupportDir, pythonBin, pool.SourceTicket, pool.Candidates, promptEnv...)
		if err != nil {
			return mergePrepareErrMsg{err: err}
		}
		result, err := codex.RunPrompt(context.Background(), pack.Prompt, pack.Schema, nil)
		if err != nil {
			return mergePrepareErrMsg{err: err}
		}
		normalized, err := triage.NormalizeMergePromptPackResult(context.Background(), codex.CustomerSupportDir, pythonBin, result.Output, pool.Candidates, promptEnv...)
		if err != nil {
			return mergePrepareErrMsg{err: err}
		}
		_ = m.workCache.AppendCodexRun(ticket.ID, map[string]any{
			"kind":       "merge",
			"ticket_id":  ticket.ID,
			"usage":      result.Usage,
			"created_at": time.Now().UTC().Format(time.RFC3339),
		})
		return mergePreparedMsg{ticketID: ticket.ID, suggestions: normalized.Suggestions, recommendedTargetID: normalized.RecommendedTargetID}
	}
}

func (m App) activeTicket() (types.Ticket, bool) {
	if m.state == detailView && m.detail.ticket != nil {
		return *m.detail.ticket, true
	}
	if m.state == kanbanView {
		if t := m.kanban.selectedTicket(); t != nil {
			return *t, true
		}
	}
	if len(m.list.items) > 0 && m.list.cursor >= 0 && m.list.cursor < len(m.list.items) {
		return m.list.items[m.list.cursor], true
	}
	return types.Ticket{}, false
}

func (m App) startDraftForActiveTicket() (App, tea.Cmd) {
	if m.draftBusy {
		return m, nil
	}
	ticket, ok := m.activeTicket()
	if !ok {
		return m, nil
	}
	m.draftBusy = true
	m.draftErr = nil
	return m, m.generateDraft(ticket)
}

func (m App) startMergeForActiveTicket() (App, tea.Cmd) {
	if m.mergeBusy {
		return m, nil
	}
	ticket, ok := m.activeTicket()
	if !ok {
		return m, nil
	}
	m.mergeBusy = true
	m.mergeErr = nil
	return m, m.generateMergeSuggestions(ticket)
}

func (m App) openAssetsFolderForActiveTicket() (App, tea.Cmd) {
	ticket, ok := m.activeTicket()
	if ok {
		if dir := m.ticketWorkDir(ticket.ID); dir != "" && m.openPath != nil {
			m.openPath(dir)
		}
	}
	return m, nil
}

func (m App) openFirstEditableField() (App, tea.Cmd) {
	ticket, ok := m.activeTicket()
	if !m.operatorPaneVisible() || !ok || m.operator.ticket == nil || m.operator.ticket.ID != ticket.ID {
		return m, nil
	}
	for _, row := range m.operator.fieldRows() {
		if row.Editable {
			return m.openInlineFieldEdit(row)
		}
	}
	return m, nil
}

func (m App) operatorPaneVisible() bool {
	return m.state == splitView && m.showDetail && m.operatorPanelWidth() > 0
}

func (m App) ticketWorkDir(ticketID int64) string {
	dir, err := m.workCache.EnsureTicketDir(ticketID)
	if err != nil {
		return ""
	}
	return dir
}

func (m App) viewWithMouse(content string) tea.View {
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = m.windowTitle()
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Auto-collapse to list-only on narrow terminals
		if m.width < 120 && m.state == splitView {
			m.state = listView
			m.showDetail = false
		}

		// Auto-expand to split view on wide terminals
		if m.width >= 120 && m.state == listView && !m.showDetail {
			m.state = splitView
			m.showDetail = true
		}

		// Kanban too narrow — switch back to list
		if m.width < 40 && m.state == kanbanView {
			m.state = listView
		}

		// Update kanban dimensions
		m.kanban.width = m.width
		m.kanban.height = m.height
		m.kanban.recomputeVisible()
		m.kanban.clampCursor()

		var cmds []tea.Cmd
		var cmd tea.Cmd

		// Send panel-appropriate sizes
		listMsg := tea.WindowSizeMsg{Width: m.listPanelWidth(), Height: msg.Height}
		m.list, cmd = m.list.Update(listMsg)
		cmds = append(cmds, cmd)

		if m.state == splitView && m.showDetail {
			detailMsg := tea.WindowSizeMsg{Width: m.detailPanelWidth(), Height: msg.Height}
			m.detail, cmd = m.detail.Update(detailMsg)
			cmds = append(cmds, cmd)
			m.operator.setSize(m.operatorPanelWidth(), msg.Height)
		} else {
			m.detail, cmd = m.detail.Update(msg)
			cmds = append(cmds, cmd)
		}

		m.actions, cmd = m.actions.Update(msg)
		cmds = append(cmds, cmd)
		m.searchM, cmd = m.searchM.Update(msg)
		cmds = append(cmds, cmd)
		m.gotoM, cmd = m.gotoM.Update(msg)
		cmds = append(cmds, cmd)
		m.cmdPalette, cmd = m.cmdPalette.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case tea.KeyPressMsg:
		if m.operator.fieldEditActive() {
			return m.handleInlineFieldKey(msg)
		}

		// Global quit — but not when in input mode
		if m.actions.mode == actionNone && !m.searchM.active && !m.gotoM.active && !m.cmdPalette.active {
			if key.Matches(msg, keys.Quit) {
				return m, tea.Quit
			}

			// ctrl+p: command palette
			if key.Matches(msg, keys.CommandPalette) {
				hasItems := len(m.list.items) > 0
				cmd := m.cmdPalette.open(m.state, m.focus, m.showDetail, m.list.hasMore, hasItems, m.perms)
				return m, cmd
			}

			if key.Matches(msg, keys.PauseTimer) {
				m.operator.pauseResumeTimer()
				return m, nil
			}

			if key.Matches(msg, keys.ResetTimer) {
				m.operator.resetTimer()
				return m, nil
			}

			if key.Matches(msg, keys.Draft) {
				return m.startDraftForActiveTicket()
			}

			if key.Matches(msg, keys.Merge) {
				return m.startMergeForActiveTicket()
			}

			// Tab: toggle focus in split view
			if key.Matches(msg, keys.Tab) && m.state == splitView && m.showDetail {
				if m.focus == focusList {
					m.focus = focusDetail
				} else {
					m.focus = focusList
				}
				return m, nil
			}

			// w: toggle kanban view
			if key.Matches(msg, keys.ToggleKanban) && (m.state == listView || m.state == splitView || m.state == kanbanView) {
				return m.toggleKanbanView()
			}

			// v: toggle detail panel
			if key.Matches(msg, keys.ToggleDetail) && (m.state == splitView || m.state == listView) {
				cmd := m.toggleDetailPanel()
				return m, cmd
			}

			// m: toggle my tickets filter
			if key.Matches(msg, keys.MyTickets) && (m.state == listView || m.state == splitView || m.state == kanbanView) {
				return m.toggleMyTickets()
			}

			// Esc handling for split view
			if key.Matches(msg, keys.Back) && m.state == splitView {
				if m.focus == focusDetail {
					m.focus = focusList
					return m, nil
				}
				if m.list.searchQuery != "" {
					m.list.searchQuery = ""
					m.list.loading = true
					return m, tea.Batch(m.list.spinner.Tick, m.list.loadTickets())
				}
				return m, nil
			}

			// Clear search results on esc in list view
			if key.Matches(msg, keys.Back) && m.state == listView && m.list.searchQuery != "" {
				m.list.searchQuery = ""
				m.list.loading = true
				return m, tea.Batch(m.list.spinner.Tick, m.list.loadTickets())
			}

			// Clear search results on esc in kanban view
			if key.Matches(msg, keys.Back) && m.state == kanbanView && m.list.searchQuery != "" {
				m.list.searchQuery = ""
				m.list.loading = true
				return m, tea.Batch(m.list.spinner.Tick, m.list.loadTickets())
			}
		}

	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if mouse.Button == tea.MouseLeft {
			m.mouseClickPending = true
			return m.handleMouseClick(mouse.X, mouse.Y)
		}

	case tea.MouseReleaseMsg:
		mouse := msg.Mouse()
		if mouse.Button == tea.MouseLeft {
			if m.mouseClickPending {
				m.mouseClickPending = false
				return m, nil
			}
			m.mouseClickPending = false
			return m.handleMouseClick(mouse.X, mouse.Y)
		}

	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		return m.handleMouseWheel(mouse.X, mouse.Y, mouse.Button)
	}

	// Route to active action overlay first
	if m.actions.mode != actionNone {
		switch msg.(type) {
		case tea.KeyPressMsg, spinner.TickMsg, ticketUpdatedMsg, actionErrMsg, mergePreviewMsg, ccAutocompleteMsg, ccAutocompleteErrMsg:
			var cmd tea.Cmd
			m.actions, cmd = m.actions.Update(msg)
			if updated, ok := msg.(ticketUpdatedMsg); ok {
				if updated.warning != nil {
					m.mergeErr = updated.warning
				}
				m.operator.resetTimer()
				m.list.loading = true
				return m, tea.Batch(cmd, m.list.spinner.Tick, m.list.loadTickets())
			}
			return m, cmd
		}
	}

	// Route to command palette overlay
	if m.cmdPalette.active {
		if _, ok := msg.(tea.KeyPressMsg); ok {
			var cmd tea.Cmd
			m.cmdPalette, cmd = m.cmdPalette.Update(msg)
			return m, cmd
		}
	}

	// Route to search overlay
	if m.searchM.active {
		if _, ok := msg.(tea.KeyPressMsg); ok {
			var cmd tea.Cmd
			m.searchM, cmd = m.searchM.Update(msg)
			return m, cmd
		}
	}

	// Route to goto overlay
	if m.gotoM.active {
		if _, ok := msg.(tea.KeyPressMsg); ok {
			var cmd tea.Cmd
			m.gotoM, cmd = m.gotoM.Update(msg)
			return m, cmd
		}
	}

	// Handle cross-cutting messages
	switch msg := msg.(type) {
	case imageOpenMsg:
		if m.openPath != nil {
			m.openPath(msg.url)
		}
		return m, nil

	case currentUserMsg:
		m.currentUser = msg.user
		m.perms = permissions.FromUser(msg.user)
		return m, nil

	case ticketFieldsLoadedMsg:
		m.operator.setTicketFields(msg.fields)
		return m, nil

	case inlineFieldUpdatedMsg:
		m.operator.cancelFieldEdit()
		if msg.ticket != nil {
			if m.operator.ticket != nil && m.operator.ticket.ID == msg.ticket.ID {
				m.operator.ticket = msg.ticket
			}
			if m.detail.ticket != nil && m.detail.ticket.ID == msg.ticket.ID {
				m.detail.ticket = msg.ticket
				if m.detail.ready {
					m.detail.viewport.SetContent(m.detail.renderContent())
				}
			}
		}
		m.list.loading = true
		cmds := []tea.Cmd{m.list.spinner.Tick, m.list.loadTickets()}
		if m.state == splitView && m.showDetail && msg.ticketID > 0 {
			m.detail = newDetailModel(m.tickets)
			m.detail.expectedID = msg.ticketID
			m.detail.width = m.detailPanelWidth()
			m.detail.height = m.height
			cmds = append(cmds, m.detail.spinner.Tick, m.detail.loadTicket(msg.ticketID), m.detail.loadAudits(msg.ticketID))
		}
		return m, tea.Batch(cmds...)

	case inlineFieldErrMsg:
		m.operator.fieldEdit.submitting = false
		m.operator.fieldEdit.err = msg.err
		return m, nil

	case operatorTickMsg:
		return m, operatorTick()

	case draftGeneratedMsg:
		m.draftBusy = false
		m.draftErr = nil
		var current types.Ticket
		if m.detail.ticket != nil && m.detail.ticket.ID == msg.ticketID {
			current = *m.detail.ticket
		} else {
			for _, ticket := range m.list.items {
				if ticket.ID == msg.ticketID {
					current = ticket
					break
				}
			}
		}
		existingTotal := triage.ExistingTotalSeconds(current)
		var cmd tea.Cmd
		m.actions, cmd = m.actions.openApproval(
			msg.ticketID,
			m.perms,
			msg.output.Answer,
			msg.output.RecommendedStatus,
			current.Status,
			m.operator.elapsedSeconds(),
			existingTotal,
			current.UpdatedAt,
			msg.output.ReasoningSummary,
		)
		return m, cmd

	case draftErrMsg:
		m.draftBusy = false
		m.draftErr = msg.err
		return m, nil

	case mergePreparedMsg:
		m.mergeBusy = false
		m.mergeErr = nil
		var cmd tea.Cmd
		m.actions, cmd = m.actions.openMerge(msg.ticketID, msg.suggestions, msg.recommendedTargetID)
		return m, cmd

	case mergePrepareErrMsg:
		m.mergeBusy = false
		m.mergeErr = msg.err
		return m, nil

	case ticketLoadedMsg:
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		m.operator.setTicket(msg.ticket, msg.users, msg.organizations, len(m.detail.imageAttachments))
		if m.state == detailView {
			return m, tea.Batch(cmd, m.updateWindowTitle())
		}
		return m, cmd

	case auditsLoadedMsg:
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		m.operator.imageCount = len(m.detail.imageAttachments)
		cmds := []tea.Cmd{cmd}
		if m.detail.ticket != nil {
			cmds = append(cmds, m.prepareAssets(m.detail.ticket.ID, msg.audits))
		}
		return m, tea.Batch(cmds...)

	case assetsPreparedMsg:
		if msg.err == nil && (m.detail.ticket == nil || msg.ticketID == m.detail.ticket.ID) {
			m.operator.setAssets(msg.manifest, msg.analysis)
		}
		return m, nil

	case countdownTickMsg:
		if !m.list.autoRefresh {
			return m, nil
		}
		m.list.refreshCountdown--
		if m.list.refreshCountdown <= 0 {
			if (m.state == listView || m.state == splitView || m.state == kanbanView) && m.list.searchQuery == "" && !m.list.loading {
				return m, m.list.loadTicketsForRefresh()
			}
			m.list.refreshCountdown = refreshIntervalSeconds
		}
		return m, scheduleCountdownTick()

	case refreshLoadedMsg:
		m.list.loading = false
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds := []tea.Cmd{cmd, m.updateWindowTitle()}
		// Reload detail if in split view
		if m.state == splitView && m.showDetail {
			if loadCmd := m.loadDetailForCursor(); loadCmd != nil {
				cmds = append(cmds, loadCmd)
			}
		}
		// Rebuild kanban columns
		if m.state == kanbanView {
			m.kanban.rebuildColumns(m.list.items)
		}
		// Ring bell when new tickets found
		if m.list.lastRefreshNewCount > 0 {
			cmds = append(cmds, ringBell())
		}
		return m, tea.Batch(cmds...)

	case ticketsLoadedMsg:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds := []tea.Cmd{cmd, m.updateWindowTitle()}
		cmds = append(cmds, m.reloadDetailIfVisible()...)
		if m.state == kanbanView {
			m.kanban.rebuildColumns(m.list.items)
		}
		return m, tea.Batch(cmds...)

	case searchResultsMsg:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds := []tea.Cmd{cmd, m.updateWindowTitle()}
		cmds = append(cmds, m.reloadDetailIfVisible()...)
		if m.state == kanbanView {
			m.kanban.rebuildColumns(m.list.items)
		}
		return m, tea.Batch(cmds...)

	case moreTicketsLoadedMsg:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		if m.state == kanbanView {
			m.kanban.rebuildColumns(m.list.items)
		}
		return m, cmd

	case moreSearchResultsMsg:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		if m.state == kanbanView {
			m.kanban.rebuildColumns(m.list.items)
		}
		return m, cmd

	case cursorChangedMsg:
		m.operator.focusTicketID(msg.id)
		if m.state == splitView && m.showDetail {
			m.cursorSeq++
			seq := m.cursorSeq
			id := msg.id
			return m, tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg {
				return cursorSettledMsg{seq: seq, id: id}
			})
		}
		return m, nil

	case cursorSettledMsg:
		if msg.seq != m.cursorSeq {
			return m, nil // stale — user moved again
		}
		if m.state == splitView && m.showDetail {
			if m.detail.ticket != nil && m.detail.ticket.ID == msg.id {
				return m, nil
			}
			m.detail = newDetailModel(m.tickets)
			m.detail.expectedID = msg.id
			m.detail.width = m.detailPanelWidth()
			m.detail.height = m.height
			return m, tea.Batch(m.detail.spinner.Tick, m.detail.loadTicket(msg.id), m.detail.loadAudits(msg.id))
		}
		return m, nil

	case showDetailMsg:
		delete(m.list.newTicketIDs, msg.id)
		m.prevState = m.state
		if m.state == splitView {
			// If detail already has this ticket, just switch to full-screen
			if m.detail.ticket != nil && m.detail.ticket.ID == msg.id {
				m.state = detailView
				m.detail.width = m.width
				m.detail.height = m.height
				m.detail.viewport.SetWidth(m.width - 4)
				m.detail.viewport.SetHeight(m.height - 6)
				m.detail.viewport.SetContent(m.detail.renderContent())
				return m, m.updateWindowTitle()
			}
		}
		m.state = detailView
		m.detail = newDetailModel(m.tickets)
		m.detail.width = m.width
		m.detail.height = m.height
		return m, tea.Batch(m.detail.spinner.Tick, m.detail.loadTicket(msg.id), m.detail.loadAudits(msg.id))

	case goBackMsg:
		if m.prevState == kanbanView {
			m.state = kanbanView
			return m, m.updateWindowTitle()
		}
		if m.showDetail {
			m.state = splitView
			m.focus = focusList
			// Resize detail to panel dimensions
			m.detail.width = m.detailPanelWidth()
			m.detail.height = m.height
			if m.detail.ready {
				m.detail.viewport.SetWidth(m.detail.width - 4)
				m.detail.viewport.SetHeight(m.detail.height - 6)
				m.detail.viewport.SetContent(m.detail.renderContent())
			}
			// Resize list to panel width
			listMsg := tea.WindowSizeMsg{Width: m.listPanelWidth(), Height: m.height}
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(listMsg)
			return m, tea.Batch(cmd, m.updateWindowTitle())
		}
		m.state = listView
		return m, m.updateWindowTitle()

	case searchDoneMsg:
		m.list.searchQuery = msg.query
		m.list.loading = true
		return m, tea.Batch(m.list.spinner.Tick, m.list.doSearch(msg.query))

	case searchCancelMsg:
		if m.list.searchQuery != "" {
			m.list.searchQuery = ""
			m.list.loading = true
			return m, tea.Batch(m.list.spinner.Tick, m.list.loadTickets())
		}
		return m, m.updateWindowTitle()

	case gotoDoneMsg:
		return m, func() tea.Msg { return showDetailMsg{id: msg.id} }

	case gotoCancelMsg:
		return m, nil

	case cmdPaletteActionMsg:
		return m.handlePaletteAction(msg.action)
	}

	// Route to active view
	switch m.state {
	case splitView:
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			// Toggle chart
			if key.Matches(msg, keys.ToggleChart) {
				m.list.showChart = !m.list.showChart
				return m, nil
			}
			// Toggle tags column
			if key.Matches(msg, keys.ToggleTags) {
				m.list.showTags = !m.list.showTags
				return m, nil
			}
			// Action keys work regardless of focus
			if len(m.list.items) > 0 {
				t := m.list.items[m.list.cursor]
				switch {
				case key.Matches(msg, keys.Refresh):
					m.list.autoRefresh = !m.list.autoRefresh
					if m.list.autoRefresh {
						m.list.refreshCountdown = refreshIntervalSeconds
						return m, scheduleCountdownTick()
					}
					m.list.newTicketIDs = make(map[int64]bool)
					return m, nil
				case key.Matches(msg, keys.ManualRefresh):
					if !m.list.loading {
						m.list.loading = true
						cmds := []tea.Cmd{m.list.spinner.Tick, m.list.loadTicketsForRefresh()}
						if m.list.autoRefresh {
							m.list.refreshCountdown = refreshIntervalSeconds
						}
						return m, tea.Batch(cmds...)
					}
				case key.Matches(msg, keys.Search):
					var cmd tea.Cmd
					m.searchM, cmd = m.searchM.open()
					return m, cmd
				case key.Matches(msg, keys.GoTo):
					var cmd tea.Cmd
					m.gotoM, cmd = m.gotoM.open()
					return m, cmd
				case key.Matches(msg, keys.Comment):
					var cmd tea.Cmd
					m.actions, cmd = m.actions.openComment(t.ID, m.perms)
					return m, cmd
				case key.Matches(msg, keys.Status):
					if !m.perms.CanChangeStatus {
						return m, nil
					}
					m.actions = m.actions.openStatus(t.ID, t.Status)
					return m, nil
				case key.Matches(msg, keys.Priority):
					m.actions = m.actions.openPriority(t.ID, t.Priority)
					return m, nil
				case key.Matches(msg, keys.Open):
					if m.openPath != nil {
						m.openPath(fmt.Sprintf("https://%s.zendesk.com/agent/tickets/%d", m.subdomain, t.ID))
					}
					return m, nil
				case key.Matches(msg, keys.Enter):
					return m, func() tea.Msg {
						return showDetailMsg{id: t.ID}
					}
				}
			} else {
				// No items but still handle search/refresh/goto
				switch {
				case key.Matches(msg, keys.Search):
					var cmd tea.Cmd
					m.searchM, cmd = m.searchM.open()
					return m, cmd
				case key.Matches(msg, keys.GoTo):
					var cmd tea.Cmd
					m.gotoM, cmd = m.gotoM.open()
					return m, cmd
				case key.Matches(msg, keys.Refresh):
					m.list.autoRefresh = !m.list.autoRefresh
					if m.list.autoRefresh {
						m.list.refreshCountdown = refreshIntervalSeconds
						return m, scheduleCountdownTick()
					}
					return m, nil
				case key.Matches(msg, keys.ManualRefresh):
					if !m.list.loading {
						m.list.loading = true
						return m, tea.Batch(m.list.spinner.Tick, m.list.loadTicketsForRefresh())
					}
				}
			}

			// Route navigation keys to focused panel
			if m.focus == focusDetail {
				var cmd tea.Cmd
				m.detail, cmd = m.detail.Update(msg)
				return m, cmd
			}
			if m.focus == focusOperator {
				return m, nil
			}
			// focusList: route to list
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		// Non-key messages: route to both
		var cmds []tea.Cmd
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
		m.detail, cmd = m.detail.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case listView:
		// Check for action keys before routing to list
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			// Toggle chart
			if key.Matches(msg, keys.ToggleChart) {
				m.list.showChart = !m.list.showChart
				return m, nil
			}
			// Toggle tags column
			if key.Matches(msg, keys.ToggleTags) {
				m.list.showTags = !m.list.showTags
				return m, nil
			}
			switch {
			case key.Matches(msg, keys.Refresh):
				m.list.autoRefresh = !m.list.autoRefresh
				if m.list.autoRefresh {
					m.list.refreshCountdown = refreshIntervalSeconds
					return m, scheduleCountdownTick()
				}
				m.list.newTicketIDs = make(map[int64]bool)
				return m, nil
			case key.Matches(msg, keys.ManualRefresh):
				if !m.list.loading {
					m.list.loading = true
					cmds := []tea.Cmd{m.list.spinner.Tick, m.list.loadTicketsForRefresh()}
					if m.list.autoRefresh {
						m.list.refreshCountdown = refreshIntervalSeconds
					}
					return m, tea.Batch(cmds...)
				}
			case key.Matches(msg, keys.Search):
				var cmd tea.Cmd
				m.searchM, cmd = m.searchM.open()
				return m, cmd
			case key.Matches(msg, keys.GoTo):
				var cmd tea.Cmd
				m.gotoM, cmd = m.gotoM.open()
				return m, cmd
			case key.Matches(msg, keys.Comment):
				if len(m.list.items) > 0 {
					t := m.list.items[m.list.cursor]
					var cmd tea.Cmd
					m.actions, cmd = m.actions.openComment(t.ID, m.perms)
					return m, cmd
				}
			case key.Matches(msg, keys.Status):
				if !m.perms.CanChangeStatus {
					return m, nil
				}
				if len(m.list.items) > 0 {
					t := m.list.items[m.list.cursor]
					m.actions = m.actions.openStatus(t.ID, t.Status)
					return m, nil
				}
			case key.Matches(msg, keys.Priority):
				if len(m.list.items) > 0 {
					t := m.list.items[m.list.cursor]
					m.actions = m.actions.openPriority(t.ID, t.Priority)
					return m, nil
				}
			case key.Matches(msg, keys.Open):
				if len(m.list.items) > 0 {
					t := m.list.items[m.list.cursor]
					if m.openPath != nil {
						m.openPath(fmt.Sprintf("https://%s.zendesk.com/agent/tickets/%d", m.subdomain, t.ID))
					}
					return m, nil
				}
			}
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd

	case detailView:
		// Check for action keys before routing to detail
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			if key.Matches(msg, keys.GoTo) {
				var cmd tea.Cmd
				m.gotoM, cmd = m.gotoM.open()
				return m, cmd
			}
			if m.detail.ticket != nil {
				switch {
				case key.Matches(msg, keys.Comment):
					var cmd tea.Cmd
					m.actions, cmd = m.actions.openComment(m.detail.ticket.ID, m.perms)
					return m, cmd
				case key.Matches(msg, keys.Status):
					if !m.perms.CanChangeStatus {
						return m, nil
					}
					m.actions = m.actions.openStatus(m.detail.ticket.ID, m.detail.ticket.Status)
					return m, nil
				case key.Matches(msg, keys.Priority):
					m.actions = m.actions.openPriority(m.detail.ticket.ID, m.detail.ticket.Priority)
					return m, nil
				case key.Matches(msg, keys.Open):
					if m.openPath != nil {
						m.openPath(fmt.Sprintf("https://%s.zendesk.com/agent/tickets/%d", m.subdomain, m.detail.ticket.ID))
					}
					return m, nil
				}
			}
		}
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd

	case kanbanView:
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			// Action keys using selected ticket
			if t := m.kanban.selectedTicket(); t != nil {
				switch {
				case key.Matches(msg, keys.Comment):
					var cmd tea.Cmd
					m.actions, cmd = m.actions.openComment(t.ID, m.perms)
					return m, cmd
				case key.Matches(msg, keys.Status):
					if !m.perms.CanChangeStatus {
						return m, nil
					}
					m.actions = m.actions.openStatus(t.ID, t.Status)
					return m, nil
				case key.Matches(msg, keys.Priority):
					m.actions = m.actions.openPriority(t.ID, t.Priority)
					return m, nil
				case key.Matches(msg, keys.Open):
					if m.openPath != nil {
						m.openPath(fmt.Sprintf("https://%s.zendesk.com/agent/tickets/%d", m.subdomain, t.ID))
					}
					return m, nil
				}
			}

			switch {
			case key.Matches(msg, keys.Search):
				var cmd tea.Cmd
				m.searchM, cmd = m.searchM.open()
				return m, cmd
			case key.Matches(msg, keys.GoTo):
				var cmd tea.Cmd
				m.gotoM, cmd = m.gotoM.open()
				return m, cmd
			case key.Matches(msg, keys.Refresh):
				m.list.autoRefresh = !m.list.autoRefresh
				if m.list.autoRefresh {
					m.list.refreshCountdown = refreshIntervalSeconds
					return m, scheduleCountdownTick()
				}
				m.list.newTicketIDs = make(map[int64]bool)
				return m, nil
			case key.Matches(msg, keys.ManualRefresh):
				if !m.list.loading {
					m.list.loading = true
					cmds := []tea.Cmd{m.list.spinner.Tick, m.list.loadTicketsForRefresh()}
					if m.list.autoRefresh {
						m.list.refreshCountdown = refreshIntervalSeconds
					}
					return m, tea.Batch(cmds...)
				}
			case key.Matches(msg, keys.NextPage):
				if m.list.hasMore && !m.list.loadingMore {
					m.list.loadingMore = true
					if m.list.searchQuery != "" {
						return m, m.list.loadMoreSearch()
					}
					return m, m.list.loadMoreTickets()
				}
			}

			// Route navigation to kanban model
			var cmd tea.Cmd
			m.kanban, cmd = m.kanban.Update(msg)
			return m, cmd
		}

		// Non-key messages: route to list (for spinner ticks etc.)
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *App) toggleDetailPanel() tea.Cmd {
	m.showDetail = !m.showDetail
	if m.showDetail {
		m.state = splitView
		listMsg := tea.WindowSizeMsg{Width: m.listPanelWidth(), Height: m.height}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(listMsg)
		cmds := []tea.Cmd{cmd}
		if loadCmd := m.loadDetailForCursor(); loadCmd != nil {
			cmds = append(cmds, loadCmd)
		}
		return tea.Batch(cmds...)
	}
	m.state = listView
	m.focus = focusList
	listMsg := tea.WindowSizeMsg{Width: m.width, Height: m.height}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(listMsg)
	return cmd
}

func (m *App) toggleKanbanView() (tea.Model, tea.Cmd) {
	if m.state == kanbanView {
		// Return to previous list/split state
		if m.showDetail {
			m.state = splitView
			m.focus = focusList
			listMsg := tea.WindowSizeMsg{Width: m.listPanelWidth(), Height: m.height}
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(listMsg)
			cmds := []tea.Cmd{cmd, m.updateWindowTitle()}
			if loadCmd := m.loadDetailForCursor(); loadCmd != nil {
				cmds = append(cmds, loadCmd)
			}
			return m, tea.Batch(cmds...)
		}
		m.state = listView
		return m, m.updateWindowTitle()
	}
	// Enter kanban view
	m.kanban.width = m.width
	m.kanban.height = m.height
	m.kanban.rebuildColumns(m.list.items)
	m.state = kanbanView
	return m, m.updateWindowTitle()
}

func (m *App) toggleMyTickets() (tea.Model, tea.Cmd) {
	if m.currentUser == nil || m.currentUser.Email == "" {
		return m, nil
	}
	myQuery := "assignee:" + m.currentUser.Email
	if m.list.searchQuery == myQuery {
		// Toggle off: clear filter and reload all tickets
		m.list.searchQuery = ""
		m.list.loading = true
		return m, tea.Batch(m.list.spinner.Tick, m.list.loadTickets())
	}
	// Toggle on: search for my tickets
	m.list.searchQuery = myQuery
	m.list.loading = true
	return m, tea.Batch(m.list.spinner.Tick, m.list.doSearch(myQuery))
}

func (m *App) handlePaletteAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "quit":
		return m, tea.Quit
	case "enter":
		if m.state == kanbanView {
			if t := m.kanban.selectedTicket(); t != nil {
				id := t.ID
				return m, func() tea.Msg { return showDetailMsg{id: id} }
			}
		} else if len(m.list.items) > 0 {
			id := m.list.items[m.list.cursor].ID
			return m, func() tea.Msg { return showDetailMsg{id: id} }
		}
	case "goto":
		var cmd tea.Cmd
		m.gotoM, cmd = m.gotoM.open()
		return m, cmd
	case "search":
		var cmd tea.Cmd
		m.searchM, cmd = m.searchM.open()
		return m, cmd
	case "open":
		var id int64
		if m.state == detailView && m.detail.ticket != nil {
			id = m.detail.ticket.ID
		} else if m.state == kanbanView {
			if t := m.kanban.selectedTicket(); t != nil {
				id = t.ID
			}
		} else if len(m.list.items) > 0 {
			id = m.list.items[m.list.cursor].ID
		}
		if id > 0 {
			if m.openPath != nil {
				m.openPath(fmt.Sprintf("https://%s.zendesk.com/agent/tickets/%d", m.subdomain, id))
			}
		}
		return m, nil
	case "comment":
		var id int64
		if m.state == detailView && m.detail.ticket != nil {
			id = m.detail.ticket.ID
		} else if m.state == kanbanView {
			if t := m.kanban.selectedTicket(); t != nil {
				id = t.ID
			}
		} else if len(m.list.items) > 0 {
			id = m.list.items[m.list.cursor].ID
		}
		if id > 0 {
			var cmd tea.Cmd
			m.actions, cmd = m.actions.openComment(id, m.perms)
			return m, cmd
		}
	case "draft":
		return m.startDraftForActiveTicket()
	case "merge":
		return m.startMergeForActiveTicket()
	case "assets":
		return m.openAssetsFolderForActiveTicket()
	case "edit-field":
		return m.openFirstEditableField()
	case "status":
		var id int64
		var status string
		if m.state == detailView && m.detail.ticket != nil {
			id = m.detail.ticket.ID
			status = m.detail.ticket.Status
		} else if m.state == kanbanView {
			if t := m.kanban.selectedTicket(); t != nil {
				id = t.ID
				status = t.Status
			}
		} else if len(m.list.items) > 0 {
			t := m.list.items[m.list.cursor]
			id = t.ID
			status = t.Status
		}
		if id > 0 {
			m.actions = m.actions.openStatus(id, status)
			return m, nil
		}
	case "priority":
		var id int64
		var priority string
		if m.state == detailView && m.detail.ticket != nil {
			id = m.detail.ticket.ID
			priority = m.detail.ticket.Priority
		} else if m.state == kanbanView {
			if t := m.kanban.selectedTicket(); t != nil {
				id = t.ID
				priority = t.Priority
			}
		} else if len(m.list.items) > 0 {
			t := m.list.items[m.list.cursor]
			id = t.ID
			priority = t.Priority
		}
		if id > 0 {
			m.actions = m.actions.openPriority(id, priority)
			return m, nil
		}
	case "my-tickets":
		return m.toggleMyTickets()
	case "toggle-kanban":
		return m.toggleKanbanView()
	case "toggle-detail":
		cmd := m.toggleDetailPanel()
		return m, cmd
	case "toggle-chart":
		m.list.showChart = !m.list.showChart
		return m, nil
	case "toggle-tags":
		m.list.showTags = !m.list.showTags
		return m, nil
	case "toggle-focus":
		if m.state == splitView && m.showDetail {
			if m.focus == focusList {
				m.focus = focusDetail
			} else {
				m.focus = focusList
			}
		}
		return m, nil
	case "refresh":
		if !m.list.loading {
			m.list.loading = true
			cmds := []tea.Cmd{m.list.spinner.Tick, m.list.loadTicketsForRefresh()}
			if m.list.autoRefresh {
				m.list.refreshCountdown = refreshIntervalSeconds
			}
			return m, tea.Batch(cmds...)
		}
	case "auto-refresh":
		m.list.autoRefresh = !m.list.autoRefresh
		if m.list.autoRefresh {
			m.list.refreshCountdown = refreshIntervalSeconds
			return m, scheduleCountdownTick()
		}
		m.list.newTicketIDs = make(map[int64]bool)
		return m, nil
	case "load-more":
		if m.list.hasMore && !m.list.loading {
			m.list.loadingMore = true
			if m.list.searchQuery != "" {
				return m, m.list.loadMoreSearch()
			}
			return m, m.list.loadMoreTickets()
		}
	}
	return m, nil
}

func (m App) handleMouseClick(x, y int) (tea.Model, tea.Cmd) {
	if m.actions.mode != actionNone {
		return m.handleActionMouseClick(x, y)
	}
	if m.cmdPalette.active {
		return m.handlePaletteMouseClick(x, y)
	}
	if m.searchM.active || m.gotoM.active {
		return m, nil
	}
	region, ok := findHitRegion(m.hitRegions(), x, y)
	if !ok {
		return m, nil
	}
	if region.Disabled {
		if region.Reason != "" {
			m.notice = region.Reason
		}
		return m, nil
	}
	switch region.Action {
	case hitPaneList:
		m.focus = focusList
		return m, nil
	case hitPaneDetail:
		m.focus = focusDetail
		return m, nil
	case hitPaneOperator:
		m.focus = focusOperator
		return m, nil
	case hitQueueRow:
		return m.selectQueueIndex(region.TicketIndex)
	case hitFieldEdit:
		if !m.operatorPaneVisible() || !m.operatorMatchesActiveTicket() {
			return m, nil
		}
		row, ok := m.operator.fieldRowByID(region.FieldID)
		if !ok || !row.Editable {
			return m, nil
		}
		return m.openInlineFieldEdit(row)
	case hitAssetsFolder, hitAssetFile:
		if !m.operatorMatchesActiveTicket() {
			return m, nil
		}
		if region.Action == hitAssetsFolder && region.Path == "" && region.TicketID > 0 {
			region.Path = m.ticketWorkDir(region.TicketID)
		}
		if region.Path != "" && m.openPath != nil {
			m.openPath(region.Path)
		}
		return m, nil
	case hitCommand:
		return m.handleMouseCommand(region.Command)
	}
	return m, nil
}

func (m App) handleMouseWheel(x, y int, button tea.MouseButton) (tea.Model, tea.Cmd) {
	if m.actions.mode != actionNone {
		if button == tea.MouseWheelUp {
			return m.forwardActionKey(tea.KeyUp)
		}
		if button == tea.MouseWheelDown {
			return m.forwardActionKey(tea.KeyDown)
		}
		return m, nil
	}
	if m.cmdPalette.active {
		if button == tea.MouseWheelUp {
			return m.forwardPaletteKey(tea.KeyUp)
		}
		if button == tea.MouseWheelDown {
			return m.forwardPaletteKey(tea.KeyDown)
		}
		return m, nil
	}
	if m.searchM.active || m.gotoM.active {
		return m, nil
	}
	region, ok := findHitRegion(m.paneHitRegions(), x, y)
	if !ok {
		return m, nil
	}
	switch region.Action {
	case hitPaneList:
		if button == tea.MouseWheelUp {
			return m.forwardListKey(tea.KeyUp)
		}
		if button == tea.MouseWheelDown {
			return m.forwardListKey(tea.KeyDown)
		}
	case hitPaneDetail:
		if button == tea.MouseWheelUp {
			return m.forwardDetailKey(tea.KeyUp)
		}
		if button == tea.MouseWheelDown {
			return m.forwardDetailKey(tea.KeyDown)
		}
	}
	return m, nil
}

func (m App) handleActionMouseClick(x, y int) (tea.Model, tea.Cmd) {
	region, ok := findHitRegion(m.actionHitRegions(), x, y)
	if !ok {
		return m, nil
	}
	switch region.Action {
	case hitActionCancel:
		return m.forwardActionKey(tea.KeyEscape)
	case hitActionToggle:
		return m.forwardActionKey(tea.KeyTab)
	case hitActionSubmit:
		if m.actions.mode == actionApproval || m.actions.mode == actionMerge {
			return m.forwardActionSubmit()
		}
		if m.actions.mode == actionField || m.actions.mode == actionComment {
			return m.forwardActionSubmit()
		}
		return m.forwardActionKey(tea.KeyEnter)
	case hitActionUp:
		return m.forwardActionKey(tea.KeyUp)
	case hitActionDown:
		return m.forwardActionKey(tea.KeyDown)
	case hitActionOption:
		return m.handleActionOptionClick(region.TicketIndex)
	}
	return m, nil
}

func (m App) handleActionOptionClick(index int) (tea.Model, tea.Cmd) {
	switch m.actions.mode {
	case actionMerge:
		if index >= 0 && index < len(m.actions.mergeSuggestions) {
			m.actions.mergeSelection = index
			m.actions.textarea.SetValue(fmt.Sprint(m.actions.mergeSuggestions[index].ID))
			m.actions.mergePreviewReady = false
		}
	case actionStatus:
		if index >= 0 && index < len(validStatuses) {
			m.actions.statusIdx = index
		}
	case actionPriority:
		if index >= 0 && index < len(validPriorities) {
			m.actions.prioIdx = index
		}
	}
	return m, nil
}

func (m App) handlePaletteMouseClick(x, y int) (tea.Model, tea.Cmd) {
	region, ok := findHitRegion(m.actionHitRegions(), x, y)
	if !ok {
		return m, nil
	}
	switch region.Action {
	case hitActionCancel:
		return m.forwardPaletteKey(tea.KeyEscape)
	case hitActionSubmit:
		return m.forwardPaletteKey(tea.KeyEnter)
	case hitActionUp:
		return m.forwardPaletteKey(tea.KeyUp)
	case hitActionDown:
		return m.forwardPaletteKey(tea.KeyDown)
	case hitActionOption:
		if region.TicketIndex >= 0 && region.TicketIndex < len(m.cmdPalette.filtered) {
			m.cmdPalette.cursor = region.TicketIndex
			return m.forwardPaletteKey(tea.KeyEnter)
		}
	}
	return m, nil
}

func (m App) forwardActionKey(code rune) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyPressMsg{Code: code})
}

func (m App) forwardActionSubmit() (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
}

func (m App) forwardPaletteKey(code rune) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyPressMsg{Code: code})
}

func (m App) forwardListKey(code rune) (tea.Model, tea.Cmd) {
	m.focus = focusList
	return m.Update(tea.KeyPressMsg{Code: code})
}

func (m App) forwardDetailKey(code rune) (tea.Model, tea.Cmd) {
	m.focus = focusDetail
	return m.Update(tea.KeyPressMsg{Code: code})
}

func (m App) handleMouseCommand(command string) (tea.Model, tea.Cmd) {
	switch command {
	case "open", "draft", "merge", "refresh", "load-more":
		return m.handlePaletteAction(command)
	case "commands":
		hasItems := len(m.list.items) > 0
		cmd := m.cmdPalette.open(m.state, m.focus, m.showDetail, m.list.hasMore, hasItems, m.perms)
		return m, cmd
	case "assets":
		return m.openAssetsFolderForActiveTicket()
	case "pause":
		m.operator.pauseResumeTimer()
		return m, nil
	case "reset":
		m.operator.resetTimer()
		return m, nil
	case "edit-field":
		return m.openFirstEditableField()
	}
	return m, nil
}

func (m App) operatorMatchesActiveTicket() bool {
	if m.operator.ticket == nil {
		return false
	}
	ticket, ok := m.activeTicket()
	return ok && ticket.ID == m.operator.ticket.ID
}

func (m App) openInlineFieldEdit(row operatorFieldRow) (App, tea.Cmd) {
	if !m.operatorPaneVisible() || m.operator.ticket == nil {
		return m, nil
	}
	m.focus = focusOperator
	cmd := m.operator.openFieldEdit(m.operator.ticket.ID, row)
	return m, cmd
}

func (m App) handleInlineFieldKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.operator.fieldEdit.submitting {
		return m, nil
	}
	switch {
	case key.Matches(msg, keys.Back):
		m.operator.cancelFieldEdit()
		return m, nil
	case key.Matches(msg, keys.Submit):
		if strings.TrimSpace(m.operator.fieldEdit.input.Value()) == "" && !m.operator.fieldEdit.clearArmed {
			m.operator.fieldEdit.clearArmed = true
			m.operator.fieldEdit.err = fmt.Errorf("field will be cleared; press ctrl+s again to confirm")
			return m, nil
		}
		m.operator.fieldEdit.submitting = true
		m.operator.fieldEdit.err = nil
		return m, m.submitInlineFieldEdit()
	default:
		var cmd tea.Cmd
		m.operator.fieldEdit.input, cmd = m.operator.fieldEdit.input.Update(msg)
		if strings.TrimSpace(m.operator.fieldEdit.input.Value()) != "" {
			m.operator.fieldEdit.clearArmed = false
		}
		return m, cmd
	}
}

func (m App) submitInlineFieldEdit() tea.Cmd {
	edit := m.operator.fieldEdit
	ticketID := edit.ticketID
	fieldID := edit.fieldID
	valueText := strings.TrimSpace(edit.input.Value())
	value, parseErr := parseTicketFieldValue(edit.fieldType, valueText)
	tickets := m.tickets
	return func() tea.Msg {
		if parseErr != nil {
			return inlineFieldErrMsg{err: parseErr}
		}
		if tickets == nil {
			return inlineFieldErrMsg{err: fmt.Errorf("ticket service unavailable")}
		}
		ticket, err := tickets.Update(context.Background(), ticketID, &types.UpdateTicketRequest{
			CustomFields: []types.CustomField{{ID: fieldID, Value: value}},
		})
		if err != nil {
			return inlineFieldErrMsg{err: err}
		}
		return inlineFieldUpdatedMsg{ticketID: ticketID, ticket: ticket}
	}
}

func (m App) selectQueueIndex(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.list.items) {
		return m, nil
	}
	wasSelected := index == m.list.cursor
	ticketID := m.list.items[index].ID
	detailAlreadyLoaded := m.detail.ticket != nil && m.detail.ticket.ID == ticketID
	m.focus = focusList
	var cursorCmd tea.Cmd
	m.list, cursorCmd = m.list.setCursorWithoutCursorChanged(index)
	delete(m.list.newTicketIDs, ticketID)
	if wasSelected && detailAlreadyLoaded && (m.state == splitView || m.state == detailView) {
		return m, cursorCmd
	}
	m.operator.focusTicketID(ticketID)
	if m.state == listView {
		if m.width >= 120 {
			m.state = splitView
			m.showDetail = true
		} else {
			m.state = detailView
		}
	}
	if m.state == splitView && m.showDetail {
		m.detail = newDetailModel(m.tickets)
		m.detail.expectedID = ticketID
		m.detail.width = m.detailPanelWidth()
		m.detail.height = m.height
		return m, tea.Batch(cursorCmd, m.detail.spinner.Tick, m.detail.loadTicket(ticketID), m.detail.loadAudits(ticketID))
	}
	if m.state == detailView {
		m.detail = newDetailModel(m.tickets)
		m.detail.expectedID = ticketID
		m.detail.width = m.width
		m.detail.height = m.height
		return m, tea.Batch(cursorCmd, m.detail.spinner.Tick, m.detail.loadTicket(ticketID), m.detail.loadAudits(ticketID))
	}
	return m, cursorCmd
}

func (m App) paneHitRegions() []hitRegion {
	if m.width <= 0 || m.height <= 0 {
		return nil
	}
	contentX := 2
	contentY := 2
	contentHeight := m.height - 3
	if contentHeight < contentY {
		contentHeight = m.height - 1
	}
	switch m.state {
	case splitView:
		listWidth := m.listPanelWidth()
		detailWidth := m.detailPanelWidth()
		operatorWidth := m.operatorPanelWidth()
		regions := []hitRegion{
			{Action: hitPaneList, X1: contentX, Y1: contentY, X2: contentX + listWidth - 1, Y2: contentHeight},
			{Action: hitPaneDetail, X1: contentX + listWidth + 1, Y1: contentY, X2: contentX + listWidth + detailWidth, Y2: contentHeight},
		}
		if operatorWidth > 0 {
			startX := contentX + listWidth + detailWidth + 2
			regions = append(regions, hitRegion{Action: hitPaneOperator, X1: startX, Y1: contentY, X2: startX + operatorWidth - 1, Y2: contentHeight})
		}
		return regions
	case listView:
		return []hitRegion{{Action: hitPaneList, X1: contentX, Y1: contentY, X2: m.width - 1, Y2: contentHeight}}
	case detailView:
		return []hitRegion{{Action: hitPaneDetail, X1: contentX, Y1: contentY, X2: m.width - 1, Y2: contentHeight}}
	}
	return nil
}

func (m App) hitRegions() []hitRegion {
	regions := m.paneHitRegions()
	contentX := 2
	contentY := 2
	if (m.state == splitView || m.state == listView) && !m.list.loading {
		listWidth := m.listPanelWidth()
		start, end := m.list.visibleWindow()
		rowY := contentY + 2
		for i := start; i < end; i++ {
			regions = append(regions, hitRegion{
				Action:      hitQueueRow,
				X1:          contentX,
				Y1:          rowY + (i - start),
				X2:          contentX + listWidth - 1,
				Y2:          rowY + (i - start),
				TicketIndex: i,
				TicketID:    m.list.items[i].ID,
			})
		}
	}
	if m.state == splitView && m.showDetail && m.operatorPanelWidth() > 0 {
		listWidth := m.listPanelWidth()
		detailWidth := m.detailPanelWidth()
		operatorX := contentX + listWidth + detailWidth + 2
		regions = append(regions, m.operator.hitRegions(operatorX, contentY, m.operatorPanelWidth(), "")...)
	}
	regions = append(regions, m.commandHitRegions()...)
	return regions
}

func (m App) commandHitRegions() []hitRegion {
	if m.height <= 0 || m.width <= 0 {
		return nil
	}
	commands := []struct {
		label   string
		command string
	}{
		{"open", "open"},
		{"field", "edit-field"},
		{"assets", "assets"},
		{"draft", "draft"},
		{"merge", "merge"},
		{"refresh", "refresh"},
		{"more", "load-more"},
		{"pause", "pause"},
		{"reset", "reset"},
		{"commands", "commands"},
	}
	x := 1
	y := m.height - 2
	regions := make([]hitRegion, 0, len(commands))
	for _, item := range commands {
		w := len(item.label)
		regions = append(regions, hitRegion{
			Action:  hitCommand,
			X1:      x,
			Y1:      y,
			X2:      x + w - 1,
			Y2:      y + 1,
			Command: item.command,
		})
		x += w + 2
	}
	return regions
}

func (m App) actionHitRegions() []hitRegion {
	if m.width <= 0 || m.height <= 0 {
		return nil
	}
	if m.cmdPalette.active {
		return m.commandPaletteHitRegions()
	}
	specs := m.actions.buttonSpecs()
	if len(specs) == 0 {
		return nil
	}
	overlay := m.actions.View()
	overlayHeight := lipgloss.Height(overlay)
	lineWidth := 0
	for i, spec := range specs {
		if i > 0 {
			lineWidth += 2
		}
		lineWidth += len("[ " + spec.Label + " ]")
	}
	x := (m.width - lineWidth) / 2
	y := (m.height-overlayHeight)/2 + overlayHeight - 3
	if y < 0 {
		y = 0
	}
	regions := make([]hitRegion, 0, len(specs))
	for _, spec := range specs {
		label := "[ " + spec.Label + " ]"
		regions = append(regions, hitRegion{
			Action: spec.Action,
			X1:     x,
			Y1:     y,
			X2:     x + len(label) - 1,
			Y2:     y,
		})
		x += len(label) + 2
	}
	regions = append(regions, m.actionOptionHitRegions()...)
	return regions
}

func (m App) commandPaletteHitRegions() []hitRegion {
	overlay := m.cmdPalette.View()
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := lipgloss.Height(overlay)
	left := (m.width - overlayWidth) / 2
	top := (m.height - overlayHeight) / 2
	if left < 0 {
		left = 0
	}
	if top < 0 {
		top = 0
	}
	regions := []hitRegion{
		{Action: hitActionCancel, X1: left + overlayWidth - 8, Y1: top, X2: left + overlayWidth - 2, Y2: top + 2},
	}

	listTop := top + cmdPaletteBorderSize + cmdPalettePaddingY + cmdPaletteListContentOffset
	for _, row := range m.cmdPalette.visibleCommandRows() {
		y := listTop + row.line
		regions = append(regions, hitRegion{
			Action:      hitActionOption,
			X1:          left + cmdPaletteBorderSize + cmdPalettePaddingX,
			Y1:          y,
			X2:          left + overlayWidth - cmdPaletteBorderSize - cmdPalettePaddingX - 1,
			Y2:          y,
			TicketIndex: row.index,
		})
	}
	return regions
}

func (m App) actionOptionHitRegions() []hitRegion {
	if m.actions.mode == actionNone || m.width <= 0 || m.height <= 0 {
		return nil
	}
	overlay := m.actions.View()
	overlayHeight := lipgloss.Height(overlay)
	overlayWidth := lipgloss.Width(overlay)
	left := (m.width - overlayWidth) / 2
	top := (m.height - overlayHeight) / 2
	if left < 0 {
		left = 0
	}
	if top < 0 {
		top = 0
	}
	x1 := left + 1
	x2 := left + overlayWidth - 2
	switch m.actions.mode {
	case actionMerge:
		if len(m.actions.mergeSuggestions) == 0 {
			return nil
		}
		startY := top + 5
		regions := make([]hitRegion, 0, len(m.actions.mergeSuggestions))
		for i := range m.actions.mergeSuggestions {
			regions = append(regions, hitRegion{
				Action:      hitActionOption,
				X1:          x1,
				Y1:          startY + i,
				X2:          x2,
				Y2:          startY + i,
				TicketIndex: i,
			})
		}
		return regions
	case actionStatus:
		return pickerOptionHitRegions(top, x1, x2, validStatuses)
	case actionPriority:
		return pickerOptionHitRegions(top, x1, x2, validPriorities)
	default:
		return nil
	}
}

func pickerOptionHitRegions(top, x1, x2 int, options []string) []hitRegion {
	regions := make([]hitRegion, 0, len(options))
	startY := top + 4
	for i := range options {
		regions = append(regions, hitRegion{
			Action:      hitActionOption,
			X1:          x1,
			Y1:          startY + i,
			X2:          x2,
			Y2:          startY + i,
			TicketIndex: i,
		})
	}
	return regions
}

func (m App) View() tea.View {
	// Overlay: action modal
	if m.actions.mode != actionNone {
		overlay := m.actions.View()
		content := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
		return m.viewWithMouse(content)
	}

	// Overlay: command palette
	if m.cmdPalette.active {
		overlay := m.cmdPalette.View()
		content := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
		return m.viewWithMouse(content)
	}

	var content string

	// Goto overlay (shown above list when active)
	if m.gotoM.active {
		content = m.gotoM.View() + "\n\n"
		switch m.state {
		case splitView:
			content += m.renderSplitView()
		case kanbanView:
			content += m.kanban.View()
		case detailView:
			content += m.detail.View()
		default:
			content += m.list.View()
		}
	} else if m.searchM.active {
		content = m.searchM.View() + "\n\n"
		switch m.state {
		case splitView:
			content += m.renderSplitView()
		case kanbanView:
			content += m.kanban.View()
		default:
			content += m.list.View()
		}
	} else {
		switch m.state {
		case listView:
			content = m.list.View()
		case detailView:
			content = m.detail.View()
		case splitView:
			content = m.renderSplitView()
		case kanbanView:
			content = m.kanban.View()
		}
	}

	// Help bar at the bottom
	helpText := m.helpBar()
	help := helpBarStyle.Width(m.width).Padding(0, 1).Render(helpText)

	// Layout: content takes remaining space, help bar at bottom
	contentHeight := m.height - lipgloss.Height(help) - 1
	styledContent := lipgloss.NewStyle().
		Height(contentHeight).
		MaxHeight(contentHeight).
		Padding(2, 2).
		Render(content)

	return m.viewWithMouse(styledContent + "\n" + help)
}

func (m App) renderSplitView() string {
	listWidth := m.listPanelWidth()
	detailWidth := m.detailPanelWidth()
	operatorWidth := m.operatorPanelWidth()

	listContent := m.list.View()
	detailContent := m.detail.ViewPanel()
	operatorContent := m.operator.View()

	// Apply focus indicator
	listPanel := lipgloss.NewStyle().Width(listWidth).Render(listContent)
	detailPanel := lipgloss.NewStyle().Width(detailWidth).Render(detailContent)
	operatorPanel := lipgloss.NewStyle().Width(operatorWidth).Render(operatorContent)

	if m.focus == focusList {
		listPanel = focusBorderStyle.Width(listWidth).Render(listContent)
	} else if m.focus == focusDetail {
		detailPanel = focusBorderStyle.Width(detailWidth).Render(detailContent)
	} else if m.focus == focusOperator && operatorWidth > 0 {
		operatorPanel = focusBorderStyle.Width(operatorWidth).Render(operatorContent)
	}

	divider := m.renderDivider()

	if operatorWidth > 0 {
		return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, divider, detailPanel, divider, operatorPanel)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, divider, detailPanel)
}

func (m App) renderDivider() string {
	height := m.height - 4
	if height < 1 {
		height = 1
	}
	divider := strings.Repeat("│\n", height-1) + "│"
	return dividerStyle.Render(divider)
}

func (m App) helpBar() string {
	var left string
	switch m.state {
	case listView:
		left = "↑↓ navigate  enter view  d draft  M merge  / search  ctrl+p commands  q quit"
	case detailView:
		if len(m.detail.imageAttachments) > 0 {
			left = "↑↓ scroll  d draft  M merge  i images  esc back  ctrl+p commands  q quit"
		} else {
			left = "↑↓ scroll  d draft  M merge  esc back  ctrl+p commands  q quit"
		}
	case splitView:
		if m.focus == focusList {
			left = "↑↓ navigate  enter view  d draft  M merge  tab focus  ctrl+p commands  q quit"
		} else if len(m.detail.imageAttachments) > 0 {
			left = "↑↓ scroll  d draft  M merge  i images  tab focus  esc back  ctrl+p commands  q quit"
		} else {
			left = "↑↓ scroll  d draft  M merge  tab focus  esc back  ctrl+p commands  q quit"
		}
	case kanbanView:
		left = "←→ columns  ↑↓ navigate  enter view  d draft  M merge  w list  ctrl+p commands  q quit"
	}

	if m.draftBusy {
		left = "Generating draft with codex exec...  " + left
	} else if m.draftErr != nil {
		left = errorStyle.Render("Draft error: "+m.draftErr.Error()) + "  " + left
	}
	if m.mergeBusy {
		left = "Ranking merge targets with codex exec...  " + left
	} else if m.mergeErr != nil {
		left = errorStyle.Render("Merge error: "+m.mergeErr.Error()) + "  " + left
	}
	if m.notice != "" {
		left = dimStyle.Render("Notice: "+m.notice) + "  " + left
	}
	mouseActions := "open  field  assets  draft  merge  refresh  more  pause  reset  commands"
	if left != "" {
		left = mouseActions + "  |  " + left
	} else {
		left = mouseActions
	}

	if m.currentUser == nil || m.width == 0 {
		return left
	}

	userInfo := m.currentUser.Email
	if userInfo == "" {
		userInfo = m.currentUser.Name
	}
	if userInfo == "" {
		return left
	}
	if m.version != "" {
		userInfo += "  " + m.version
	}

	// Right-align user info with padding
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(userInfo) - 2
	if gap < 2 {
		return left
	}
	return left + strings.Repeat(" ", gap) + userInfo
}
