package tui

import (
	"testing"

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

func TestFormatElapsed(t *testing.T) {
	assert.Equal(t, "0:05", formatElapsed(5))
	assert.Equal(t, "1:01", formatElapsed(61))
	assert.Equal(t, "1:00:01", formatElapsed(3601))
}
