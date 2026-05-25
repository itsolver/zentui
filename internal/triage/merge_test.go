package triage

import (
	"testing"
	"time"

	"github.com/itsolver/zentui/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeMergeRankingSortsBlocksClosedAndRecommendsHighConfidence(t *testing.T) {
	now := time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC)
	ranking := MergeRanking{RankedCandidates: []struct {
		TicketID       int64  `json:"ticket_id"`
		RelevanceScore int    `json:"relevance_score"`
		Rationale      string `json:"rationale"`
	}{
		{TicketID: 10, RelevanceScore: 40, Rationale: "weak"},
		{TicketID: 11, RelevanceScore: 91, Rationale: "same issue"},
		{TicketID: 12, RelevanceScore: 100, Rationale: "closed"},
	}}
	candidates := []types.Ticket{
		{ID: 10, Subject: "Low", Status: "open", UpdatedAt: now},
		{ID: 11, Subject: "Strong", Status: "pending", UpdatedAt: now.Add(-time.Minute)},
		{ID: 12, Subject: "Closed", Status: "closed", UpdatedAt: now},
	}

	out, recommended := NormalizeMergeRanking(ranking, candidates, 5)

	require.Len(t, out, 2)
	assert.Equal(t, int64(11), out[0].Ticket.ID)
	assert.Equal(t, "same issue", out[0].Rationale)
	assert.Equal(t, int64(11), recommended)
}

func TestIsRecentSolvedTarget(t *testing.T) {
	now := time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC)

	assert.True(t, IsRecentSolvedTarget(types.Ticket{Status: "solved", UpdatedAt: now.Add(-23 * time.Hour)}, now))
	assert.False(t, IsRecentSolvedTarget(types.Ticket{Status: "solved", UpdatedAt: now.Add(-25 * time.Hour)}, now))
	assert.False(t, IsRecentSolvedTarget(types.Ticket{Status: "open", UpdatedAt: now}, now))
}
