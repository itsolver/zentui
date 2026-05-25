package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/itsolver/zentui/internal/triage"
	"github.com/itsolver/zentui/internal/types"
)

type operatorTickMsg struct{}

type ticketFieldsLoadedMsg struct {
	fields []types.TicketField
}

type operatorModel struct {
	width       int
	height      int
	ticket      *types.Ticket
	users       map[int64]types.User
	orgs        map[int64]types.Organization
	fieldLabels map[int64]string
	imageCount  int
	assets      []triage.AssetRecord
	analysis    map[string]triage.ImageAnalysis
	timer       triage.TicketTimer
	timerPaused bool
}

func newOperatorModel() operatorModel {
	return operatorModel{
		users:       map[int64]types.User{},
		orgs:        map[int64]types.Organization{},
		fieldLabels: map[int64]string{},
		analysis:    map[string]triage.ImageAnalysis{},
	}
}

func operatorTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return operatorTickMsg{} })
}

func (m *operatorModel) setSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *operatorModel) focusTicketID(ticketID int64) {
	m.timer.Focus(ticketID, time.Now())
	m.timerPaused = false
}

func (m *operatorModel) setTicket(ticket types.Ticket, users []types.User, orgs []types.Organization, imageCount int) {
	m.ticket = &ticket
	m.users = make(map[int64]types.User, len(users))
	for _, user := range users {
		m.users[user.ID] = user
	}
	m.orgs = make(map[int64]types.Organization, len(orgs))
	for _, org := range orgs {
		m.orgs[org.ID] = org
	}
	m.imageCount = imageCount
	m.focusTicketID(ticket.ID)
}

func (m *operatorModel) setTicketFields(fields []types.TicketField) {
	m.fieldLabels = make(map[int64]string, len(fields))
	for _, field := range fields {
		m.fieldLabels[field.ID] = field.Title
	}
}

func (m *operatorModel) setAssets(manifest triage.Manifest, analysis map[string]triage.ImageAnalysis) {
	m.assets = manifest.Assets
	m.analysis = analysis
	count := 0
	for _, asset := range manifest.Assets {
		if !asset.Skipped {
			count++
		}
	}
	if count > 0 {
		m.imageCount = count
	}
}

func (m *operatorModel) pauseResumeTimer() {
	now := time.Now()
	if m.timer.Running() {
		m.timer.Pause(now)
		m.timerPaused = true
		return
	}
	m.timer.Resume(now)
	m.timerPaused = false
}

func (m *operatorModel) resetTimer() {
	m.timer.Reset(time.Now())
}

func (m operatorModel) elapsedSeconds() int {
	return m.timer.ElapsedSeconds(time.Now())
}

func (m operatorModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Operator") + "\n\n")
	if m.ticket == nil {
		b.WriteString(subtitleStyle.Render("Focus a ticket"))
		return b.String()
	}

	b.WriteString(m.renderLine("Timer", formatElapsed(m.elapsedSeconds())))
	if m.timerPaused {
		b.WriteString(commentTimeStyle.Render("paused") + "\n")
	}
	b.WriteString("\n")

	if requester, ok := m.users[m.ticket.RequesterID]; ok {
		b.WriteString(headerStyle.Render("Requester") + "\n")
		b.WriteString(m.renderLine("Name", requester.Name))
		b.WriteString(m.renderLine("Email", requester.Email))
		b.WriteString("\n")
	}

	if org, ok := m.orgs[m.ticket.OrganizationID]; ok {
		b.WriteString(headerStyle.Render("Organisation") + "\n")
		b.WriteString(m.renderLine("Name", org.Name))
		if org.Details != "" {
			b.WriteString(m.renderLine("Details", org.Details))
		}
		b.WriteString("\n")
	}

	b.WriteString(headerStyle.Render("Assets") + "\n")
	b.WriteString(m.renderLine("Images", fmt.Sprintf("%d", m.imageCount)))
	for i, asset := range m.assets {
		if i >= 3 {
			b.WriteString(dimStyle.Render("...") + "\n")
			break
		}
		if asset.Skipped {
			b.WriteString(m.renderLine(asset.Filename, asset.SkipReason))
			continue
		}
		b.WriteString(m.renderLine(asset.Filename, asset.LocalPath))
		if obs, ok := m.analysis[asset.SHA256]; ok {
			prefix := "AI"
			if obs.IsSignatureOrLogo {
				prefix = "AI low"
			}
			b.WriteString(m.renderLine(prefix, obs.Summary))
		}
	}
	b.WriteString("\n")

	if len(m.ticket.CustomFields) > 0 {
		b.WriteString(headerStyle.Render("Fields") + "\n")
		for _, field := range m.ticket.CustomFields {
			if field.Value == nil || fmt.Sprint(field.Value) == "" {
				continue
			}
			label := m.fieldLabels[field.ID]
			if label == "" {
				label = fmt.Sprintf("%d", field.ID)
			}
			b.WriteString(m.renderLine(label, fmt.Sprint(field.Value)))
		}
	}

	b.WriteString("\n" + dimStyle.Render("d draft   M merge   P pause timer   0 reset"))
	return b.String()
}

func (m operatorModel) renderLine(label, value string) string {
	if value == "" {
		value = "-"
	}
	width := m.width - len(label) - 5
	if width < 8 {
		width = 8
	}
	runes := []rune(value)
	if len(runes) > width {
		value = string(runes[:width-1]) + "…"
	}
	return labelStyle.Render(label+":") + " " + valueStyle.Render(value) + "\n"
}

func formatElapsed(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
