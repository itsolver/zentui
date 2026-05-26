package triage

import "fmt"

func BuildMergeSourceComment(targetTicketID int64) string {
	return fmt.Sprintf("Merged into ticket #%d by support.", targetTicketID)
}

func BuildMergeTargetPublicComment(sourceTicketID, targetTicketID int64) string {
	return fmt.Sprintf(
		"We've merged request #%d into this existing request #%d so we can keep the related conversation and work together in one place.\n\nPlease reply here for any further updates on this request.",
		sourceTicketID,
		targetTicketID,
	)
}
