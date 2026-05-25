package triage

import (
	"sort"
	"time"

	"github.com/itsolver/zentui/internal/types"
)

type MergeCandidate struct {
	Ticket         types.Ticket
	RelevanceScore int
	Rationale      string
}

type MergeRanking struct {
	RankedCandidates []struct {
		TicketID       int64  `json:"ticket_id"`
		RelevanceScore int    `json:"relevance_score"`
		Rationale      string `json:"rationale"`
	} `json:"ranked_candidates"`
}

func NormalizeMergeRanking(ranking MergeRanking, candidates []types.Ticket, max int) ([]MergeCandidate, int64) {
	if max <= 0 {
		max = 5
	}

	byID := make(map[int64]struct {
		score     int
		rationale string
	}, len(ranking.RankedCandidates))
	for _, item := range ranking.RankedCandidates {
		score := item.RelevanceScore
		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}
		byID[item.TicketID] = struct {
			score     int
			rationale string
		}{score: score, rationale: item.Rationale}
	}

	out := make([]MergeCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Status == "closed" {
			continue
		}
		score := byID[candidate.ID]
		out = append(out, MergeCandidate{
			Ticket:         candidate,
			RelevanceScore: score.score,
			Rationale:      score.rationale,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RelevanceScore == out[j].RelevanceScore {
			return out[i].Ticket.UpdatedAt.After(out[j].Ticket.UpdatedAt)
		}
		return out[i].RelevanceScore > out[j].RelevanceScore
	})
	if len(out) > max {
		out = out[:max]
	}

	var recommended int64
	if len(out) > 0 && out[0].RelevanceScore >= 85 {
		recommended = out[0].Ticket.ID
	}
	return out, recommended
}

func IsRecentSolvedTarget(ticket types.Ticket, now time.Time) bool {
	return ticket.Status == "solved" && now.Sub(ticket.UpdatedAt) <= 24*time.Hour
}
