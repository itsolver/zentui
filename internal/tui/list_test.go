package tui

import (
	"testing"
	"time"

	"github.com/itsolver/zentui/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestRenderTicketRowUsesCompactStatusAndPriority(t *testing.T) {
	m := newListModel(nil, nil)
	m.width = 120
	now := time.Now()
	ticket := types.Ticket{
		ID:        123,
		Status:    "open",
		Priority:  "high",
		Subject:   "Compact ticket row",
		UpdatedAt: now.Add(-2 * time.Minute),
		CreatedAt: now.Add(-12 * 24 * time.Hour),
	}

	row := stripANSI(m.renderTicketRow(ticket, false))

	assert.Contains(t, row, "#123")
	assert.Contains(t, row, "Compact ticket row")
	assert.Contains(t, row, "2m ago")
	assert.NotContains(t, row, "12d ago")
	assert.NotContains(t, row, "open")
	assert.NotContains(t, row, "high")
}
