package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHitRegionContainsBounds(t *testing.T) {
	region := hitRegion{X1: 2, Y1: 3, X2: 8, Y2: 5}

	assert.True(t, region.contains(2, 3))
	assert.True(t, region.contains(8, 5))
	assert.False(t, region.contains(1, 3))
	assert.False(t, region.contains(2, 6))
}

func TestFindHitRegionPrefersLaterOverlap(t *testing.T) {
	regions := []hitRegion{
		{Action: hitPaneList, X1: 0, Y1: 0, X2: 20, Y2: 10},
		{Action: hitQueueRow, X1: 0, Y1: 2, X2: 20, Y2: 2, TicketIndex: 4},
	}

	region, ok := findHitRegion(regions, 5, 2)

	require.True(t, ok)
	assert.Equal(t, hitQueueRow, region.Action)
	assert.Equal(t, 4, region.TicketIndex)
}
