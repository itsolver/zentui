package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/itsolver/zentui/internal/permissions"
	"github.com/itsolver/zentui/internal/triage"

	"github.com/itsolver/zentui/internal/types"
	"github.com/itsolver/zentui/pkg/zendesk"
)

type ticketUpdatedMsg struct {
	ticket *types.Ticket
}

type actionErrMsg struct{ err error }

type mergePreviewMsg struct {
	sourceStatus  string
	targetStatus  string
	targetSubject string
	cleanupPlan   triage.RequesterCleanupPlan
}

type actionMode int

const (
	actionNone actionMode = iota
	actionComment
	actionApproval
	actionMerge
	actionStatus
	actionPriority
)

var validStatuses = []string{"new", "open", "pending", "hold", "solved"}
var validPriorities = []string{"urgent", "high", "normal", "low"}

type actionsModel struct {
	tickets             zendesk.TicketService
	users               zendesk.UserService
	ticketID            int64
	mode                actionMode
	textarea            textarea.Model
	isPublic            bool
	perms               permissions.Permissions
	statusIdx           int
	prioIdx             int
	suggestedStatus     string
	elapsedSeconds      int
	existingTotal       int
	reasoningSummary    string
	sourceTicketID      int64
	mergeSuggestions    []triage.MergeSuggestion
	mergeSelection      int
	mergeCleanupPlan    triage.RequesterCleanupPlan
	mergeCleanupEnabled bool
	mergePreviewReady   bool
	mergeSourceStatus   string
	mergeTargetStatus   string
	mergeTargetSubject  string
	submitting          bool
	err                 error
	spinner             spinner.Model
	width               int
	height              int
	current             string // current status or priority
	ccPicker            ccPickerModel
	ccFocused           bool
}

func newActionsModel(tickets zendesk.TicketService, users zendesk.UserService) actionsModel {
	ta := textarea.New()
	ta.Placeholder = "Type your comment..."
	ta.ShowLineNumbers = false
	ta.SetHeight(6)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ac("#1D4ED8", "#93C5FD"))

	return actionsModel{
		tickets:  tickets,
		users:    users,
		textarea: ta,
		isPublic: true,
		spinner:  s,
		ccPicker: newCCPickerModel(users),
	}
}

func (m actionsModel) openComment(ticketID int64, perms permissions.Permissions) (actionsModel, tea.Cmd) {
	m.ticketID = ticketID
	m.mode = actionComment
	m.perms = perms
	m.isPublic = perms.CanPublicComment
	m.err = nil
	m.ccFocused = false
	m.ccPicker = m.ccPicker.reset()
	m.textarea.Reset()
	m.textarea.Placeholder = "Type your comment..."
	m.textarea.SetHeight(6)
	return m, m.textarea.Focus()
}

func (m actionsModel) openApproval(ticketID int64, perms permissions.Permissions, body string, suggestedStatus string, currentStatus string, elapsedSeconds int, existingTotal int, reasoningSummary string) (actionsModel, tea.Cmd) {
	m.ticketID = ticketID
	m.mode = actionApproval
	m.perms = perms
	m.isPublic = perms.CanPublicComment
	m.err = nil
	m.ccFocused = false
	m.ccPicker = m.ccPicker.reset()
	m.textarea.Reset()
	m.textarea.Placeholder = "Review or edit the draft..."
	m.textarea.SetHeight(8)
	m.textarea.SetValue(body)
	m.suggestedStatus = suggestedStatus
	m.elapsedSeconds = elapsedSeconds
	m.existingTotal = existingTotal
	m.reasoningSummary = reasoningSummary
	m.statusIdx = 0
	defaultStatus := suggestedStatus
	if defaultStatus == "" {
		defaultStatus = currentStatus
	}
	for i, status := range validStatuses {
		if status == defaultStatus {
			m.statusIdx = i
			break
		}
	}
	return m, m.textarea.Focus()
}

func (m actionsModel) openMerge(sourceTicketID int64, suggestions []triage.MergeSuggestion, recommendedTargetID int64) (actionsModel, tea.Cmd) {
	m.ticketID = sourceTicketID
	m.sourceTicketID = sourceTicketID
	m.mode = actionMerge
	m.err = nil
	m.mergeSuggestions = suggestions
	m.mergeSelection = 0
	m.mergeCleanupPlan = triage.RequesterCleanupPlan{}
	m.mergeCleanupEnabled = false
	m.mergePreviewReady = false
	m.mergeSourceStatus = ""
	m.mergeTargetStatus = ""
	m.mergeTargetSubject = ""
	m.textarea.Reset()
	m.textarea.Placeholder = "Target ticket ID"
	m.textarea.SetHeight(1)
	for i, suggestion := range suggestions {
		if suggestion.ID == recommendedTargetID {
			m.mergeSelection = i
			m.textarea.SetValue(fmt.Sprint(suggestion.ID))
			break
		}
	}
	return m, m.textarea.Focus()
}

func (m actionsModel) openStatus(ticketID int64, currentStatus string) actionsModel {
	m.ticketID = ticketID
	m.mode = actionStatus
	m.current = currentStatus
	m.err = nil
	m.statusIdx = 0
	for i, s := range validStatuses {
		if s == currentStatus {
			m.statusIdx = i
			break
		}
	}
	return m
}

func (m actionsModel) openPriority(ticketID int64, currentPriority string) actionsModel {
	m.ticketID = ticketID
	m.mode = actionPriority
	m.current = currentPriority
	m.err = nil
	m.prioIdx = 0
	for i, p := range validPriorities {
		if p == currentPriority {
			m.prioIdx = i
			break
		}
	}
	return m
}

func (m actionsModel) close() actionsModel {
	m.mode = actionNone
	m.textarea.Blur()
	m.textarea.Placeholder = "Type your comment..."
	m.textarea.SetHeight(6)
	return m
}

func (m actionsModel) submitComment() tea.Cmd {
	body := m.textarea.Value()
	isPublic := m.isPublic
	ticketID := m.ticketID
	tickets := m.tickets
	collaborators := append([]types.CollaboratorEntry(nil), m.ccPicker.selected...)
	return func() tea.Msg {
		pub := isPublic
		req := &types.UpdateTicketRequest{
			Comment: &types.Comment{
				Body:   body,
				Public: &pub,
			},
		}
		if pub && len(collaborators) > 0 {
			req.AdditionalCollaborators = collaborators
		}
		ticket, err := tickets.Update(context.Background(), ticketID, req)
		if err != nil {
			return actionErrMsg{err}
		}
		return ticketUpdatedMsg{ticket: ticket}
	}
}

func (m actionsModel) submitApproval() tea.Cmd {
	body := m.textarea.Value()
	isPublic := m.isPublic
	ticketID := m.ticketID
	status := validStatuses[m.statusIdx]
	tickets := m.tickets
	elapsed := m.elapsedSeconds
	existingTotal := m.existingTotal
	return func() tea.Msg {
		req := triage.BuildApprovalUpdate(triage.ApprovalInput{
			Body:                 body,
			Public:               isPublic,
			ConfirmedStatus:      status,
			ElapsedSeconds:       elapsed,
			ExistingTotalSeconds: existingTotal,
		})
		ticket, err := tickets.Update(context.Background(), ticketID, req)
		if err != nil {
			return actionErrMsg{err}
		}
		return ticketUpdatedMsg{ticket: ticket}
	}
}

func (m actionsModel) submitMerge() tea.Cmd {
	sourceID := m.sourceTicketID
	targetText := strings.TrimSpace(m.textarea.Value())
	tickets := m.tickets
	users := m.users
	cleanupEnabled := m.mergeCleanupEnabled
	return func() tea.Msg {
		ctx := context.Background()
		targetID, err := strconv.ParseInt(targetText, 10, 64)
		if err != nil || targetID <= 0 {
			return actionErrMsg{err: fmt.Errorf("target ticket ID is required")}
		}
		if targetID == sourceID {
			return actionErrMsg{err: fmt.Errorf("cannot merge a ticket into itself")}
		}
		sourceResult, err := tickets.Get(ctx, sourceID, &types.GetTicketOptions{Include: "users"})
		if err != nil {
			return actionErrMsg{err: err}
		}
		targetResult, err := tickets.Get(ctx, targetID, &types.GetTicketOptions{Include: "users"})
		if err != nil {
			return actionErrMsg{err: err}
		}
		if !triage.IsMergeableSourceStatus(sourceResult.Ticket.Status) {
			return actionErrMsg{err: fmt.Errorf("source ticket is not mergeable in its current status")}
		}
		if !triage.IsMergeableTargetStatus(targetResult.Ticket.Status) {
			return actionErrMsg{err: fmt.Errorf("target ticket is not mergeable in its current status")}
		}
		if sourceResult.Ticket.OrganizationID != 0 && targetResult.Ticket.OrganizationID != 0 && sourceResult.Ticket.OrganizationID != targetResult.Ticket.OrganizationID {
			return actionErrMsg{err: fmt.Errorf("target ticket must be in the same organization")}
		}
		audits, err := tickets.ListAudits(ctx, sourceID, &types.ListAuditsOptions{Include: "users", SortOrder: "asc"})
		if err != nil {
			return actionErrMsg{err: err}
		}
		sourceUser := findUser(sourceResult.Users, sourceResult.Ticket.RequesterID)
		targetUser := findUser(targetResult.Users, targetResult.Ticket.RequesterID)
		cleanupPlan := triage.BuildRequesterCleanupPlan(sourceResult.Ticket, audits.Audits, sourceUser, targetResult.Ticket, targetUser)
		if cleanupEnabled {
			if _, err := triage.ExecuteRequesterCleanup(ctx, users, cleanupPlan); err != nil {
				return actionErrMsg{err: err}
			}
		}
		result, err := tickets.MergeTickets(ctx, targetID, &types.MergeTicketsRequest{
			IDs:           []int64{sourceID},
			SourceComment: fmt.Sprintf("Closing as merged into #%d.", targetID),
			TargetComment: fmt.Sprintf("Merging duplicate/follow-up ticket #%d.", sourceID),
		})
		if err != nil {
			return actionErrMsg{err: err}
		}
		if result.Ticket != nil {
			return ticketUpdatedMsg{ticket: result.Ticket}
		}
		return ticketUpdatedMsg{ticket: &types.Ticket{ID: targetID}}
	}
}

func (m actionsModel) prepareMergePreview() tea.Cmd {
	sourceID := m.sourceTicketID
	targetText := strings.TrimSpace(m.textarea.Value())
	tickets := m.tickets
	return func() tea.Msg {
		ctx := context.Background()
		targetID, err := strconv.ParseInt(targetText, 10, 64)
		if err != nil || targetID <= 0 {
			return actionErrMsg{err: fmt.Errorf("target ticket ID is required")}
		}
		if targetID == sourceID {
			return actionErrMsg{err: fmt.Errorf("cannot merge a ticket into itself")}
		}
		sourceResult, err := tickets.Get(ctx, sourceID, &types.GetTicketOptions{Include: "users"})
		if err != nil {
			return actionErrMsg{err: err}
		}
		targetResult, err := tickets.Get(ctx, targetID, &types.GetTicketOptions{Include: "users"})
		if err != nil {
			return actionErrMsg{err: err}
		}
		if !triage.IsMergeableSourceStatus(sourceResult.Ticket.Status) {
			return actionErrMsg{err: fmt.Errorf("source ticket is not mergeable in its current status")}
		}
		if !triage.IsMergeableTargetStatus(targetResult.Ticket.Status) {
			return actionErrMsg{err: fmt.Errorf("target ticket is not mergeable in its current status")}
		}
		if sourceResult.Ticket.OrganizationID != 0 && targetResult.Ticket.OrganizationID != 0 && sourceResult.Ticket.OrganizationID != targetResult.Ticket.OrganizationID {
			return actionErrMsg{err: fmt.Errorf("target ticket must be in the same organization")}
		}
		audits, err := tickets.ListAudits(ctx, sourceID, &types.ListAuditsOptions{Include: "users", SortOrder: "asc"})
		if err != nil {
			return actionErrMsg{err: err}
		}
		sourceUser := findUser(sourceResult.Users, sourceResult.Ticket.RequesterID)
		targetUser := findUser(targetResult.Users, targetResult.Ticket.RequesterID)
		plan := triage.BuildRequesterCleanupPlan(sourceResult.Ticket, audits.Audits, sourceUser, targetResult.Ticket, targetUser)
		return mergePreviewMsg{
			sourceStatus:  sourceResult.Ticket.Status,
			targetStatus:  targetResult.Ticket.Status,
			targetSubject: targetResult.Ticket.Subject,
			cleanupPlan:   plan,
		}
	}
}

func findUser(users []types.User, id int64) *types.User {
	for i := range users {
		if users[i].ID == id {
			return &users[i]
		}
	}
	return nil
}

func (m actionsModel) submitStatus() tea.Cmd {
	status := validStatuses[m.statusIdx]
	ticketID := m.ticketID
	tickets := m.tickets
	return func() tea.Msg {
		ticket, err := tickets.Update(context.Background(), ticketID, &types.UpdateTicketRequest{
			Status: status,
		})
		if err != nil {
			return actionErrMsg{err}
		}
		return ticketUpdatedMsg{ticket: ticket}
	}
}

func (m actionsModel) submitPriority() tea.Cmd {
	priority := validPriorities[m.prioIdx]
	ticketID := m.ticketID
	tickets := m.tickets
	return func() tea.Msg {
		ticket, err := tickets.Update(context.Background(), ticketID, &types.UpdateTicketRequest{
			Priority: priority,
		})
		if err != nil {
			return actionErrMsg{err}
		}
		return ticketUpdatedMsg{ticket: ticket}
	}
}

func (m actionsModel) Update(msg tea.Msg) (actionsModel, tea.Cmd) {
	if m.mode == actionNone {
		return m, nil
	}

	switch msg := msg.(type) {
	case ccAutocompleteMsg, ccAutocompleteErrMsg:
		if m.mode == actionComment && m.ccFocused {
			var cmd tea.Cmd
			m.ccPicker, cmd = m.ccPicker.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		if m.submitting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case ticketUpdatedMsg:
		m.submitting = false
		m = m.close()
		return m, nil

	case mergePreviewMsg:
		m.submitting = false
		m.mergePreviewReady = true
		m.mergeSourceStatus = msg.sourceStatus
		m.mergeTargetStatus = msg.targetStatus
		m.mergeTargetSubject = msg.targetSubject
		m.mergeCleanupPlan = msg.cleanupPlan
		m.mergeCleanupEnabled = msg.cleanupPlan.DefaultEnabled
		return m, nil

	case actionErrMsg:
		m.submitting = false
		m.err = msg.err
		return m, nil

	case tea.KeyPressMsg:
		if m.submitting {
			return m, nil
		}

		switch m.mode {
		case actionComment:
			// Route to CC picker when focused
			if m.ccFocused {
				switch {
				case key.Matches(msg, keys.AddCC):
					m.ccPicker = m.ccPicker.deactivate()
					m.ccFocused = false
					return m, m.textarea.Focus()
				default:
					var cmd tea.Cmd
					m.ccPicker, cmd = m.ccPicker.Update(msg)
					// Check if picker deactivated itself (esc)
					if !m.ccPicker.active {
						m.ccFocused = false
						return m, tea.Batch(cmd, m.textarea.Focus())
					}
					return m, cmd
				}
			}

			switch {
			case key.Matches(msg, keys.Back):
				m = m.close()
				return m, nil
			case key.Matches(msg, keys.Submit):
				if m.textarea.Value() != "" {
					m.submitting = true
					return m, tea.Batch(m.spinner.Tick, m.submitComment())
				}
			case key.Matches(msg, keys.Tab):
				if !m.perms.CanPublicComment {
					return m, nil
				}
				m.isPublic = !m.isPublic
				if !m.isPublic {
					m.ccPicker = m.ccPicker.deactivate()
					m.ccFocused = false
					m.ccPicker.selected = nil
				}
				return m, nil
			case key.Matches(msg, keys.AddCC):
				if !m.perms.CanAddCC {
					return m, nil
				}
				if m.isPublic {
					m.ccFocused = true
					m.textarea.Blur()
					var cmd tea.Cmd
					m.ccPicker, cmd = m.ccPicker.activate()
					return m, cmd
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				return m, cmd
			}

		case actionApproval:
			switch {
			case key.Matches(msg, keys.Back):
				m = m.close()
				return m, nil
			case key.Matches(msg, keys.Submit):
				if m.textarea.Value() != "" {
					m.submitting = true
					return m, tea.Batch(m.spinner.Tick, m.submitApproval())
				}
			case key.Matches(msg, keys.Tab):
				if !m.perms.CanPublicComment {
					return m, nil
				}
				m.isPublic = !m.isPublic
				return m, nil
			case key.Matches(msg, keys.Up):
				if m.statusIdx > 0 {
					m.statusIdx--
				}
				return m, nil
			case key.Matches(msg, keys.Down):
				if m.statusIdx < len(validStatuses)-1 {
					m.statusIdx++
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				return m, cmd
			}

		case actionMerge:
			switch {
			case key.Matches(msg, keys.Back):
				m = m.close()
				m.textarea.Placeholder = "Type your comment..."
				m.textarea.SetHeight(6)
				return m, nil
			case key.Matches(msg, keys.Submit):
				if strings.TrimSpace(m.textarea.Value()) != "" {
					m.submitting = true
					if !m.mergePreviewReady {
						return m, tea.Batch(m.spinner.Tick, m.prepareMergePreview())
					}
					return m, tea.Batch(m.spinner.Tick, m.submitMerge())
				}
			case key.Matches(msg, keys.Tab):
				if m.mergeCleanupPlan.Eligible {
					m.mergeCleanupEnabled = !m.mergeCleanupEnabled
				}
				return m, nil
			case key.Matches(msg, keys.Up):
				if len(m.mergeSuggestions) > 0 && m.mergeSelection > 0 {
					m.mergeSelection--
					m.textarea.SetValue(fmt.Sprint(m.mergeSuggestions[m.mergeSelection].ID))
					m.mergePreviewReady = false
				}
				return m, nil
			case key.Matches(msg, keys.Down):
				if len(m.mergeSuggestions) > 0 && m.mergeSelection < len(m.mergeSuggestions)-1 {
					m.mergeSelection++
					m.textarea.SetValue(fmt.Sprint(m.mergeSuggestions[m.mergeSelection].ID))
					m.mergePreviewReady = false
				}
				return m, nil
			case key.Matches(msg, keys.Enter):
				if len(m.mergeSuggestions) > 0 {
					m.textarea.SetValue(fmt.Sprint(m.mergeSuggestions[m.mergeSelection].ID))
					m.mergePreviewReady = false
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				m.mergePreviewReady = false
				return m, cmd
			}

		case actionStatus:
			switch {
			case key.Matches(msg, keys.Back):
				m = m.close()
				return m, nil
			case key.Matches(msg, keys.Up):
				if m.statusIdx > 0 {
					m.statusIdx--
				}
			case key.Matches(msg, keys.Down):
				if m.statusIdx < len(validStatuses)-1 {
					m.statusIdx++
				}
			case key.Matches(msg, keys.Enter):
				m.submitting = true
				return m, tea.Batch(m.spinner.Tick, m.submitStatus())
			}

		case actionPriority:
			switch {
			case key.Matches(msg, keys.Back):
				m = m.close()
				return m, nil
			case key.Matches(msg, keys.Up):
				if m.prioIdx > 0 {
					m.prioIdx--
				}
			case key.Matches(msg, keys.Down):
				if m.prioIdx < len(validPriorities)-1 {
					m.prioIdx++
				}
			case key.Matches(msg, keys.Enter):
				m.submitting = true
				return m, tea.Batch(m.spinner.Tick, m.submitPriority())
			}
		}
	}
	return m, nil
}

func (m actionsModel) View() string {
	if m.mode == actionNone {
		return ""
	}

	switch m.mode {
	case actionComment:
		return m.viewComment()
	case actionApproval:
		return m.viewApproval()
	case actionMerge:
		return m.viewMerge()
	case actionStatus:
		return m.viewPicker("Change Status", validStatuses, m.statusIdx)
	case actionPriority:
		return m.viewPicker("Change Priority", validPriorities, m.prioIdx)
	}
	return ""
}

func (m actionsModel) viewMerge() string {
	title := titleStyle.Render("Merge Ticket")
	width := m.width - 8
	if width < 50 {
		width = 50
	}
	m.textarea.SetWidth(width)
	var statusLine string
	if m.submitting {
		statusLine = "\n" + m.spinner.View() + " Checking merge..."
	} else if m.err != nil {
		statusLine = "\n" + errorStyle.Render("Error: "+m.err.Error())
	}

	var suggestions strings.Builder
	if len(m.mergeSuggestions) > 0 {
		suggestions.WriteString(headerStyle.Render("Suggestions") + "\n")
		for i, suggestion := range m.mergeSuggestions {
			pointer := "  "
			if i == m.mergeSelection {
				pointer = "> "
			}
			line := fmt.Sprintf("#%d %s %s %d%%", suggestion.ID, suggestion.Status, suggestion.Subject, suggestion.RelevanceScore)
			if suggestion.Rationale != "" {
				line += " - " + suggestion.Rationale
			}
			if i == m.mergeSelection {
				suggestions.WriteString(selectedStyle.Render(pointer+line) + "\n")
			} else {
				suggestions.WriteString(pointer + line + "\n")
			}
		}
		suggestions.WriteString("\n")
	}

	var preview strings.Builder
	if m.mergePreviewReady {
		preview.WriteString(headerStyle.Render("Confirmation") + "\n")
		preview.WriteString(labelStyle.Render("Source status:") + " " + valueStyle.Render(m.mergeSourceStatus) + "\n")
		preview.WriteString(labelStyle.Render("Target status:") + " " + valueStyle.Render(m.mergeTargetStatus) + "\n")
		preview.WriteString(labelStyle.Render("Target subject:") + " " + valueStyle.Render(m.mergeTargetSubject) + "\n")
		preview.WriteString(labelStyle.Render("Source comment:") + " " + valueStyle.Render(fmt.Sprintf("Closing as merged into #%s.", strings.TrimSpace(m.textarea.Value()))) + "\n")
		preview.WriteString(labelStyle.Render("Target comment:") + " " + valueStyle.Render(fmt.Sprintf("Merging duplicate/follow-up ticket #%d.", m.sourceTicketID)) + "\n")
		cleanup := "unavailable"
		if m.mergeCleanupPlan.Eligible {
			if m.mergeCleanupEnabled {
				cleanup = "will run"
			} else {
				cleanup = "available, disabled"
			}
		} else if m.mergeCleanupPlan.Reason != "" {
			cleanup = "skipped: " + m.mergeCleanupPlan.Reason
		}
		preview.WriteString(labelStyle.Render("Requester cleanup:") + " " + valueStyle.Render(cleanup) + "\n")
		if m.mergeCleanupPlan.PhoneNumber != "" {
			preview.WriteString(labelStyle.Render("Phone identity:") + " " + valueStyle.Render(m.mergeCleanupPlan.PhoneNumber) + "\n")
		}
		preview.WriteString("\n")
	}

	help := "ctrl+s preview"
	if m.mergePreviewReady {
		help = "ctrl+s confirm merge"
	}
	if m.mergeCleanupPlan.Eligible {
		help += "   tab toggle cleanup"
	}
	help += "   ↑↓ suggestions   enter select   esc cancel"
	content := title + "\n\n" +
		labelStyle.Render("Source:") + " " + valueStyle.Render(fmt.Sprintf("#%d", m.sourceTicketID)) + "\n" +
		suggestions.String() +
		labelStyle.Render("Target:") + "\n" + m.textarea.View() + "\n\n" +
		preview.String() +
		dimStyle.Render(help) + statusLine
	return borderStyle.Width(width + 4).Render(content)
}

func (m actionsModel) viewApproval() string {
	title := titleStyle.Render("Approve Draft")

	var publicToggle string
	if !m.perms.CanPublicComment {
		publicToggle = "[x] Internal note only (light agent)"
	} else if m.isPublic {
		publicToggle = "[x] Public reply   [ ] Internal note"
	} else {
		publicToggle = "[ ] Public reply   [x] Internal note"
	}

	width := m.width - 8
	if width < 50 {
		width = 50
	}
	m.textarea.SetWidth(width)

	status := validStatuses[m.statusIdx]
	var statusLine strings.Builder
	statusLine.WriteString(labelStyle.Render("Suggested status:") + " " + valueStyle.Render(m.suggestedStatus) + "\n")
	statusLine.WriteString(labelStyle.Render("Confirmed status:") + " " + valueStyle.Render(status) + "\n")
	if m.elapsedSeconds > 0 {
		statusLine.WriteString(labelStyle.Render("Time write:") + " " + valueStyle.Render(fmt.Sprintf("%ds this update, %ds total", m.elapsedSeconds, m.existingTotal+m.elapsedSeconds)) + "\n")
	}
	if m.reasoningSummary != "" {
		statusLine.WriteString(labelStyle.Render("AI note:") + " " + valueStyle.Render(m.reasoningSummary) + "\n")
	}

	var submitLine string
	if m.submitting {
		submitLine = "\n" + m.spinner.View() + " Posting approved update..."
	} else if m.err != nil {
		submitLine = "\n" + errorStyle.Render("Error: "+m.err.Error())
	}

	help := dimStyle.Render("ctrl+s post   esc cancel   tab public/internal   ↑↓ status")
	content := title + "\n\n" + statusLine.String() + "\n" + m.textarea.View() + "\n\n" + publicToggle + "\n\n" + help + submitLine
	return borderStyle.Width(width + 4).Render(content)
}

func (m actionsModel) viewComment() string {
	title := titleStyle.Render("Add Comment")

	var publicToggle string
	if !m.perms.CanPublicComment {
		publicToggle = "[x] Internal note only (light agent)"
	} else if m.isPublic {
		publicToggle = "[x] Public reply   [ ] Internal note"
	} else {
		publicToggle = "[ ] Public reply   [x] Internal note"
	}

	var statusLine string
	if m.submitting {
		statusLine = m.spinner.View() + " Submitting..."
	} else if m.err != nil {
		statusLine = errorStyle.Render("Error: " + m.err.Error())
	}

	var help string
	if !m.perms.CanPublicComment {
		help = dimStyle.Render("ctrl+s submit   esc cancel")
	} else {
		help = dimStyle.Render("ctrl+s submit   esc cancel   tab toggle public/internal   ctrl+a add CC")
	}

	width := m.width - 8
	if width < 40 {
		width = 40
	}
	m.textarea.SetWidth(width)
	m.ccPicker.width = width

	ccLine := m.ccPicker.viewFull(m.isPublic)

	content := title + "\n\n" +
		m.textarea.View() + "\n\n" +
		publicToggle + "\n" +
		ccLine + "\n\n" +
		help
	if statusLine != "" {
		content += "\n" + statusLine
	}

	return borderStyle.Width(width + 4).Render(content)
}

func (m actionsModel) viewPicker(title string, options []string, selected int) string {
	var b fmt.Stringer = &pickerBuilder{title: title, options: options, selected: selected, current: m.current}

	var statusLine string
	if m.submitting {
		statusLine = "\n" + m.spinner.View() + " Updating..."
	} else if m.err != nil {
		statusLine = "\n" + errorStyle.Render("Error: "+m.err.Error())
	}

	help := dimStyle.Render("↑↓ select   enter confirm   esc cancel")

	return borderStyle.Padding(1, 2).Render(b.String() + "\n\n" + help + statusLine)
}

type pickerBuilder struct {
	title    string
	options  []string
	selected int
	current  string
}

func (p *pickerBuilder) String() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(p.title) + "\n\n")
	for i, opt := range p.options {
		pointer := "  "
		if i == p.selected {
			pointer = "> "
		}
		label := opt
		if opt == p.current {
			label += " (current)"
		}
		if i == p.selected {
			b.WriteString(selectedStyle.Render(pointer+label) + "\n")
		} else {
			b.WriteString(pointer + label + "\n")
		}
	}
	return b.String()
}
