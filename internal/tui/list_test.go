package tui

import (
	"testing"
	"time"

	"github.com/itsolver/zentui/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSetCursorWithoutCursorChangedKeepsLoadMoreSideEffects(t *testing.T) {
	m := newListModel(nil, nil)
	m.loading = false
	m.hasMore = true
	m.afterCursor = "next"
	now := time.Now()
	m.items = []types.Ticket{
		{ID: 1, Subject: "First", UpdatedAt: now, CreatedAt: now},
		{ID: 2, Subject: "Second", UpdatedAt: now, CreatedAt: now},
	}

	updated, cmd := m.setCursorWithoutCursorChanged(1)

	assert.Equal(t, 1, updated.cursor)
	assert.True(t, updated.loadingMore)
	require.NotNil(t, cmd)
}
