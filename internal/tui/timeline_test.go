package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/itsolver/zentui/internal/types"
)

func boolP(b bool) *bool { return &b }

func TestBuildTimeline_CommentOnlyAudit(t *testing.T) {
	audits := []types.Audit{
		{
			ID: 1, TicketID: 1, AuthorID: 10,
			CreatedAt: time.Now(),
			Events: []types.AuditEvent{
				{ID: 1, Type: "Comment", Body: "Hello", Public: boolP(true), AuthorID: 10},
			},
		},
	}

	nodes := buildTimeline(audits)
	require.Len(t, nodes, 1, "expected 1 node")
	assert.Len(t, nodes[0].Comments, 1, "expected 1 comment")
	assert.Len(t, nodes[0].Changes, 0, "expected 0 changes")
}

func TestBuildTimeline_ChangeOnlyAudit(t *testing.T) {
	audits := []types.Audit{
		{
			ID: 1, TicketID: 1, AuthorID: 10,
			CreatedAt: time.Now(),
			Events: []types.AuditEvent{
				{ID: 1, Type: "Change", FieldName: "status", Value: "open", PreviousValue: "new"},
			},
		},
	}

	nodes := buildTimeline(audits)
	require.Len(t, nodes, 1, "expected 1 node")
	assert.Len(t, nodes[0].Comments, 0, "expected 0 comments")
	assert.Len(t, nodes[0].Changes, 1, "expected 1 change")
}

func TestBuildTimeline_MixedAudit(t *testing.T) {
	audits := []types.Audit{
		{
			ID: 1, TicketID: 1, AuthorID: 10,
			CreatedAt: time.Now(),
			Events: []types.AuditEvent{
				{ID: 1, Type: "Comment", Body: "Fixed it", Public: boolP(true), AuthorID: 10},
				{ID: 2, Type: "Change", FieldName: "status", Value: "solved", PreviousValue: "open"},
				{ID: 3, Type: "Change", FieldName: "priority", Value: "high", PreviousValue: "normal"},
			},
		},
	}

	nodes := buildTimeline(audits)
	require.Len(t, nodes, 1, "expected 1 node")
	assert.Len(t, nodes[0].Comments, 1, "expected 1 comment")
	assert.Len(t, nodes[0].Changes, 2, "expected 2 changes")
}

func TestBuildTimeline_FiltersIrrelevantEvents(t *testing.T) {
	audits := []types.Audit{
		{
			ID: 1, TicketID: 1, AuthorID: 10,
			CreatedAt: time.Now(),
			Events: []types.AuditEvent{
				{ID: 1, Type: "Change", FieldName: "custom_field_123", Value: "foo"},
				{ID: 2, Type: "Comment", Body: ""}, // empty body
			},
		},
	}

	nodes := buildTimeline(audits)
	require.Len(t, nodes, 0, "expected 0 nodes for irrelevant events")
}

func TestFilterCommentNodes(t *testing.T) {
	nodes := []TimelineNode{
		{Comments: []types.AuditEvent{{Type: "Comment", Body: "Hi"}}},
		{Changes: []types.AuditEvent{{Type: "Change", FieldName: "status"}}},
		{
			Comments: []types.AuditEvent{{Type: "Comment", Body: "Done"}},
			Changes:  []types.AuditEvent{{Type: "Change", FieldName: "status"}},
		},
	}

	filtered := filterCommentNodes(nodes)
	require.Len(t, filtered, 2, "expected 2 comment nodes")
}

func TestRenderTimeline_NonEmpty(t *testing.T) {
	now := time.Now()
	audits := []types.Audit{
		{
			ID: 1, TicketID: 1, AuthorID: 10,
			CreatedAt: now.Add(-2 * time.Hour),
			Events: []types.AuditEvent{
				{ID: 1, Type: "Comment", Body: "First message", Public: boolP(true), AuthorID: 10},
			},
		},
		{
			ID: 2, TicketID: 1, AuthorID: 20,
			CreatedAt: now.Add(-1 * time.Hour),
			Events: []types.AuditEvent{
				{ID: 2, Type: "Change", FieldName: "status", Value: "open", PreviousValue: "new"},
			},
		},
	}

	nodes := buildTimeline(audits)
	users := map[int64]types.User{
		10: {ID: 10, Name: "Alice"},
		20: {ID: 20, Name: "Bob"},
	}

	result := renderTimeline(nodes, users, 60)
	require.NotEmpty(t, result, "expected non-empty timeline render")
	assert.GreaterOrEqual(t, len(result), 20, "timeline render too short: %q", result)
}

func TestRenderConversationUsesMetadataAndInternalStyling(t *testing.T) {
	now := time.Now()
	pub := true
	internal := false
	ticket := types.Ticket{ID: 42, RequesterID: 10, AssigneeID: 20}
	users := map[int64]types.User{
		10: {ID: 10, Name: "Julie Boardman", Role: "end-user"},
		20: {ID: 20, Name: "Angus McLauchlan", Role: "agent"},
	}
	comments := []types.Comment{
		{
			ID:        1,
			Body:      "No email from Synergy?",
			Public:    &pub,
			AuthorID:  10,
			CreatedAt: now.Add(-2 * time.Hour),
			Metadata: &types.CommentMetadata{Via: &types.CommentVia{
				Channel: "email",
				Source: types.CommentViaSource{
					To: types.CommentViaParties{{Name: "IT Solver", Address: "support@example.com"}},
				},
			}},
		},
		{
			ID:        2,
			Body:      "Private context",
			Public:    &internal,
			AuthorID:  20,
			CreatedAt: now.Add(-time.Hour),
		},
	}

	result := stripANSI(renderConversation(comments, ticket, users, 80))

	assert.Contains(t, result, "Julie Boardman")
	assert.Contains(t, result, "via email")
	assert.Contains(t, result, "To: IT Solver <support@example.com>")
	assert.Contains(t, result, "No email from Synergy?")
	assert.Contains(t, result, "Angus McLauchlan")
	assert.Contains(t, result, "internal note")
	assert.Contains(t, result, "Private context")
}

func TestRenderConversationInfersAgentRecipient(t *testing.T) {
	pub := true
	ticket := types.Ticket{ID: 42, RequesterID: 10, AssigneeID: 20}
	users := map[int64]types.User{
		10: {ID: 10, Name: "Julie Boardman", Role: "end-user"},
		20: {ID: 20, Name: "Angus McLauchlan", Role: "agent"},
	}
	comments := []types.Comment{{
		ID:       1,
		Body:     "Initiated Change of Registrant",
		Public:   &pub,
		AuthorID: 20,
	}}

	result := stripANSI(renderConversation(comments, ticket, users, 80))

	assert.Contains(t, result, "To: Julie Boardman")
}

func TestDetailHeaderUsesZendeskStyleFields(t *testing.T) {
	m := detailModel{
		ticket: &types.Ticket{
			ID:             42,
			Subject:        "Invalid Registrant - Notice of Suspension",
			Status:         "pending",
			Type:           "question",
			RequesterID:    10,
			OrganizationID: 99,
		},
		users: map[int64]types.User{
			10: {ID: 10, Name: "Julie Boardman"},
		},
		organizations: map[int64]types.Organization{
			99: {ID: 99, Name: "Acorn Agencies"},
		},
		width: 120,
	}

	header := stripANSI(m.renderHeaderLine(false))

	assert.Contains(t, header, "Acorn Agencies")
	assert.Contains(t, header, "Julie Boardman")
	assert.Contains(t, header, "pending")
	assert.Contains(t, header, "question")
	assert.Contains(t, header, "#42")
	assert.Contains(t, header, "Invalid Registrant")
	assert.Contains(t, header, "[Events]")
}

func TestDetailRenderDefaultsToConversationAndTogglesEvents(t *testing.T) {
	pub := true
	m := detailModel{
		ticket: &types.Ticket{ID: 42, RequesterID: 10, AssigneeID: 20},
		users: map[int64]types.User{
			10: {ID: 10, Name: "Julie Boardman", Role: "end-user"},
			20: {ID: 20, Name: "Angus McLauchlan", Role: "agent"},
		},
		comments:       []types.Comment{{ID: 1, Body: "Conversation body", Public: &pub, AuthorID: 10}},
		commentsLoaded: true,
		timeline: []TimelineNode{{
			Audit:   types.Audit{ID: 1, AuthorID: 20, CreatedAt: time.Now()},
			Changes: []types.AuditEvent{{Type: "Change", FieldName: "status", PreviousValue: "new", Value: "open"}},
		}},
		width: 100,
	}

	conversation := stripANSI(m.renderContent())
	assert.Contains(t, conversation, "Conversation body")
	assert.NotContains(t, conversation, "Status:")

	m.toggleEvents()
	events := stripANSI(m.renderContent())
	assert.Contains(t, events, "Status:")
	assert.NotContains(t, events, "Conversation body")
	assert.Equal(t, "[Conversation]", m.toggleLabel())
}

func TestDetailFilterKeyTogglesEvents(t *testing.T) {
	m := detailModel{}

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'f'})

	assert.True(t, updated.showEvents)
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		text  string
		width int
		lines int
	}{
		{"short", 80, 1},
		{"hello world this is a test", 12, 3},
		{"line1\nline2", 80, 2},
		{"", 80, 1},
		{"https://example.com/very/long/url/that/exceeds/the/width/limit", 20, 4},
		{"before https://example.com/very/long/url/that/exceeds after", 20, 4},
	}

	for _, tt := range tests {
		result := wrapText(tt.text, tt.width)
		assert.Len(t, result, tt.lines, "wrapText(%q, %d)", tt.text, tt.width)
	}
}
