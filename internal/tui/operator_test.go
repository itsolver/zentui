package tui

import (
	"testing"

	"github.com/itsolver/zentui/internal/triage"
	"github.com/itsolver/zentui/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestOperatorModelRendersRequesterOrgFieldsAndAssets(t *testing.T) {
	m := newOperatorModel()
	m.setSize(40, 20)
	m.setTicketFields([]types.TicketField{{ID: 99, Title: "Support Plan"}})
	m.setTicket(
		types.Ticket{ID: 123, RequesterID: 1, OrganizationID: 2, CustomFields: []types.CustomField{{ID: 99, Value: "Managed"}}},
		[]types.User{{ID: 1, Name: "Alice", Email: "alice@example.com"}},
		[]types.Organization{{ID: 2, Name: "Acme"}},
		3,
	)

	view := m.View()

	assert.Contains(t, view, "Alice")
	assert.Contains(t, view, "Acme")
	assert.Contains(t, view, "Support Plan")
	assert.Contains(t, view, "3")
}

func TestOperatorModelRendersTotalTimeAndHidesNoisyFields(t *testing.T) {
	m := newOperatorModel()
	m.setSize(60, 20)
	m.setTicketFields([]types.TicketField{
		{ID: triage.TimeSpentTotalFieldID, Title: "Total time spent"},
		{ID: 100, Title: "Calendar event invite requester"},
		{ID: 101, Title: "Calendar event invite CCs"},
		{ID: 102, Title: "Support Plan"},
	})
	m.setTicket(
		types.Ticket{
			ID: 123,
			CustomFields: []types.CustomField{
				{ID: triage.TimeSpentTotalFieldID, Value: 120},
				{ID: 100, Value: "yes"},
				{ID: 101, Value: "yes"},
				{ID: 102, Value: "Managed"},
			},
		},
		nil,
		nil,
		0,
	)

	view := stripANSI(m.View())

	assert.Contains(t, view, "Time spent")
	assert.Contains(t, view, "2:00")
	assert.Contains(t, view, "Support Plan")
	assert.NotContains(t, view, "Calendar event invite requester")
	assert.NotContains(t, view, "Calendar event invite CCs")
	assert.NotContains(t, view, "Total time spent")
}

func TestFormatElapsed(t *testing.T) {
	assert.Equal(t, "0:05", formatElapsed(5))
	assert.Equal(t, "1:01", formatElapsed(61))
	assert.Equal(t, "1:00:01", formatElapsed(3601))
}

func TestOperatorModelTreatsUnknownFieldMetadataAsReadOnly(t *testing.T) {
	m := newOperatorModel()
	m.setSize(40, 20)
	m.setTicket(
		types.Ticket{ID: 123, CustomFields: []types.CustomField{{ID: 999, Value: "mystery"}}},
		nil,
		nil,
		0,
	)

	rows := m.fieldRows()

	assert.Len(t, rows, 1)
	assert.False(t, rows[0].Editable)
	assert.Equal(t, "read-only", rows[0].ReadOnly)
}
