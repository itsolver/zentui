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
	ticket := types.Ticket{
		ID:        123,
		Status:    "open",
		Priority:  "high",
		Subject:   "Compact ticket row",
		UpdatedAt: time.Now(),
		CreatedAt: time.Now(),
	}

	row := stripANSI(m.renderTicketRow(ticket, false))

	assert.Contains(t, row, "#123")
	assert.Contains(t, row, "Compact ticket row")
	assert.NotContains(t, row, "open")
	assert.NotContains(t, row, "high")
}
