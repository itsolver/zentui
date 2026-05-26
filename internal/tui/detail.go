package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/itsolver/zentui/internal/types"
	"github.com/itsolver/zentui/pkg/zendesk"
)

type ticketLoadedMsg struct {
	ticket        types.Ticket
	users         []types.User
	organizations []types.Organization
}

type auditsLoadedMsg struct {
	ticketID int64
	audits   []types.Audit
	users    []types.User
}

type commentsLoadedMsg struct {
	ticketID int64
	comments []types.Comment
	users    []types.User
}

type goBackMsg struct{}

type detailModel struct {
	tickets          zendesk.TicketService
	ticket           *types.Ticket
	users            map[int64]types.User
	organizations    map[int64]types.Organization
	comments         []types.Comment
	audits           []types.Audit
	timeline         []TimelineNode
	showEvents       bool
	commentsLoaded   bool
	viewport         viewport.Model
	loading          bool
	err              error
	spinner          spinner.Model
	width            int
	height           int
	ready            bool
	expectedID       int64
	imageAttachments []imageEntry
	imagePicker      imagePickerModel
}

func newDetailModel(tickets zendesk.TicketService) detailModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ac("#1D4ED8", "#93C5FD"))
	return detailModel{
		tickets: tickets,
		loading: true,
		spinner: s,
	}
}

func (m detailModel) loadData(id int64) tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadTicket(id), m.loadComments(id), m.loadAudits(id))
}

func (m detailModel) loadTicket(id int64) tea.Cmd {
	return func() tea.Msg {
		result, err := m.tickets.Get(context.Background(), id, &types.GetTicketOptions{
			Include: "users,organizations",
		})
		if err != nil {
			return errMsg{err}
		}
		return ticketLoadedMsg{ticket: result.Ticket, users: result.Users, organizations: result.Organizations}
	}
}

func (m detailModel) loadComments(id int64) tea.Cmd {
	return func() tea.Msg {
		page, err := m.tickets.ListComments(context.Background(), id, &types.ListCommentsOptions{
			Limit:     100,
			Include:   "users",
			SortOrder: "asc",
		})
		if err != nil {
			return errMsg{err}
		}
		return commentsLoadedMsg{ticketID: id, comments: page.Comments, users: page.Users}
	}
}

func (m detailModel) loadAudits(id int64) tea.Cmd {
	return func() tea.Msg {
		page, err := m.tickets.ListAudits(context.Background(), id, &types.ListAuditsOptions{
			Include:   "users",
			SortOrder: "asc",
		})
		if err != nil {
			return errMsg{err}
		}
		return auditsLoadedMsg{ticketID: id, audits: page.Audits, users: page.Users}
	}
}

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.ready {
			m.viewport.SetWidth(m.viewportWidth())
			m.viewport.SetHeight(m.viewportHeight())
			if m.ticket != nil {
				m.viewport.SetContent(m.renderContent())
			}
		}

	case ticketLoadedMsg:
		if m.expectedID != 0 && msg.ticket.ID != m.expectedID {
			return m, nil
		}
		m.loading = false
		m.ticket = &msg.ticket
		m.mergeUsers(msg.users)
		m.organizations = make(map[int64]types.Organization)
		for _, org := range msg.organizations {
			m.organizations[org.ID] = org
		}
		m.viewport = viewport.New(viewport.WithWidth(m.viewportWidth()), viewport.WithHeight(m.viewportHeight()))
		m.viewport.SetContent(m.renderContent())
		m.ready = true
		return m, nil

	case commentsLoadedMsg:
		if m.expectedID != 0 && msg.ticketID != m.expectedID {
			return m, nil
		}
		m.comments = msg.comments
		m.commentsLoaded = true
		m.mergeUsers(msg.users)
		if m.ready {
			m.viewport.SetContent(m.renderContent())
		}

	case auditsLoadedMsg:
		if m.expectedID != 0 && msg.ticketID != m.expectedID {
			return m, nil
		}
		m.audits = msg.audits
		m.mergeUsers(msg.users)
		m.timeline = buildTimeline(m.audits)
		m.buildImageEntries()
		if m.ready {
			m.viewport.SetContent(m.renderContent())
		}

	case errMsg:
		m.loading = false
		m.err = msg.err

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case imagePickerCloseMsg:
		m.imagePicker = m.imagePicker.close()

	case tea.KeyPressMsg:
		if m.imagePicker.active {
			var cmd tea.Cmd
			m.imagePicker, cmd = m.imagePicker.Update(msg)
			return m, cmd
		}
		switch {
		case key.Matches(msg, keys.Back):
			return m, func() tea.Msg { return goBackMsg{} }
		case key.Matches(msg, keys.FilterTimeline):
			m.toggleEvents()
			if m.ready {
				m.viewport.SetContent(m.renderContent())
			}
			return m, nil
		case key.Matches(msg, keys.Images):
			if len(m.imageAttachments) > 0 {
				m.imagePicker = m.imagePicker.open(m.imageAttachments)
				return m, nil
			}
		}
		if m.ready {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m detailModel) View() string {
	if m.loading {
		return m.spinner.View() + " Loading ticket..."
	}
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if !m.ready || m.ticket == nil {
		return ""
	}

	if m.imagePicker.active {
		return m.imagePicker.View()
	}

	header := m.renderHeaderLine(true)

	return header + "\n\n" + m.viewport.View()
}

func (m detailModel) ViewPanel() string {
	if m.loading {
		return m.spinner.View() + " Loading ticket..."
	}
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if !m.ready || m.ticket == nil {
		return subtitleStyle.Render("Select a ticket to view details")
	}

	if m.imagePicker.active {
		return m.imagePicker.View()
	}

	header := m.renderHeaderLine(false)
	return header + "\n\n" + m.viewport.View()
}

func (m detailModel) renderContent() string {
	if m.ticket == nil {
		return ""
	}
	contentWidth := m.contentWidth()
	if m.showEvents {
		if len(m.timeline) == 0 {
			return dimStyle.Render("No audit events")
		}
		return renderTimeline(m.timeline, m.users, contentWidth)
	}

	if !m.commentsLoaded {
		return dimStyle.Render("Loading conversation...")
	}
	if len(m.comments) == 0 {
		return dimStyle.Render("No comments")
	}
	return renderConversation(m.comments, *m.ticket, m.users, contentWidth)
}

func (m *detailModel) buildImageEntries() {
	m.imageAttachments = nil
	idx := 0
	for _, audit := range m.audits {
		author := timelineUserName(audit.AuthorID, m.users)
		for _, ev := range audit.Events {
			if ev.Type != "Comment" {
				continue
			}
			for _, a := range ev.Attachments {
				idx++
				if a.IsImage() {
					m.imageAttachments = append(m.imageAttachments, imageEntry{
						index:      idx,
						attachment: a,
						authorName: author,
					})
				}
			}
		}
	}
}

func (m detailModel) renderField(label, value string) string {
	return labelStyle.Render(label+":") + " " + valueStyle.Render(value) + "\n"
}

func (m detailModel) renderTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	var styled []string
	for _, t := range tags {
		styled = append(styled, tagStyle.Render(t))
	}
	return labelStyle.Render("Tags:") + " " + strings.Join(styled, " ") + "\n"
}

func (m detailModel) userName(id int64) string {
	if id == 0 {
		return dimStyle.Render("unassigned")
	}
	if u, ok := m.users[id]; ok {
		if u.Email != "" {
			return u.Name + " (" + u.Email + ")"
		}
		return u.Name
	}
	return fmt.Sprintf("User #%d", id)
}

func (m *detailModel) mergeUsers(users []types.User) {
	if m.users == nil {
		m.users = make(map[int64]types.User)
	}
	for _, u := range users {
		m.users[u.ID] = u
	}
}

func (m *detailModel) toggleEvents() {
	m.showEvents = !m.showEvents
}

func (m detailModel) viewportWidth() int {
	w := m.width - 4
	if w < 20 {
		w = 20
	}
	return w
}

func (m detailModel) viewportHeight() int {
	h := m.height - 6
	if h < 1 {
		h = 1
	}
	return h
}

func (m detailModel) contentWidth() int {
	w := m.viewportWidth()
	if w < 40 {
		w = 40
	}
	return w
}

func (m detailModel) renderHeaderLine(includeBack bool) string {
	width := m.viewportWidth()
	toggle := accentStyle.Render(m.toggleLabel())
	leftWidth := width - lipgloss.Width(toggle) - 2
	if leftWidth < 12 {
		leftWidth = width
	}

	left := m.renderTicketHeader(leftWidth)
	if includeBack {
		prefix := subtitleStyle.Render("← esc")
		leftWidth -= lipgloss.Width(prefix) + 3
		if leftWidth < 12 {
			leftWidth = 12
		}
		left = prefix + "   " + m.renderTicketHeader(leftWidth)
	}
	return alignRight(left, toggle, width)
}

func (m detailModel) toggleLabel() string {
	if m.showEvents {
		return "[Conversation]"
	}
	return "[Events]"
}

func (m detailModel) renderTicketHeader(width int) string {
	if m.ticket == nil {
		return ""
	}
	t := m.ticket
	var parts []string
	if org := m.organizationName(t.OrganizationID); org != "" {
		parts = append(parts, org)
	}
	if requester := m.userDisplayName(t.RequesterID); requester != "" {
		parts = append(parts, requester)
	}
	if t.Status != "" {
		parts = append(parts, styledStatus(t.Status))
	}
	if t.Type != "" {
		parts = append(parts, valueStyle.Render(t.Type))
	}
	parts = append(parts, valueStyle.Render(fmt.Sprintf("#%d", t.ID)))

	prefix := strings.Join(parts, "  ")
	subject := strings.ReplaceAll(strings.ReplaceAll(t.Subject, "\n", " "), "\r", "")
	subjectWidth := width - lipgloss.Width(prefix) - 2
	if subjectWidth < 8 {
		subjectWidth = 8
	}
	subject = truncateRunes(subject, subjectWidth)
	header := prefix + "  " + titleStyle.Render(subject)
	if lipgloss.Width(header) > width {
		header = valueStyle.Render(fmt.Sprintf("#%d", t.ID)) + "  " + titleStyle.Render(truncateRunes(t.Subject, width-12))
	}
	return header
}

func (m detailModel) userDisplayName(id int64) string {
	if id == 0 {
		return ""
	}
	if u, ok := m.users[id]; ok && u.Name != "" {
		return valueStyle.Render(u.Name)
	}
	return dimStyle.Render(fmt.Sprintf("User #%d", id))
}

func (m detailModel) organizationName(id int64) string {
	if id == 0 || m.organizations == nil {
		return ""
	}
	if org, ok := m.organizations[id]; ok {
		return valueStyle.Render(org.Name)
	}
	return ""
}

func alignRight(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left + " " + right
	}
	return left + strings.Repeat(" ", gap) + right
}

func truncateRunes(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}
