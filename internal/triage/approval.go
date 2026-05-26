package triage

import "github.com/itsolver/zentui/internal/types"

const (
	TimeSpentLastUpdateFieldID = int64(5398029717519)
	TimeSpentTotalFieldID      = int64(5397925840655)
)

type ApprovalInput struct {
	Body                 string
	Public               bool
	ConfirmedStatus      string
	ElapsedSeconds       int
	ExistingTotalSeconds int
	UpdatedStamp         string
}

func BuildApprovalUpdate(input ApprovalInput) *types.UpdateTicketRequest {
	req := &types.UpdateTicketRequest{
		Comment: &types.Comment{
			Body:   input.Body,
			Public: &input.Public,
		},
	}
	if input.UpdatedStamp != "" {
		req.SafeUpdate = true
		req.UpdatedStamp = input.UpdatedStamp
	}
	if input.ConfirmedStatus != "" {
		req.Status = input.ConfirmedStatus
	}
	if input.ElapsedSeconds > 0 {
		req.CustomFields = []types.CustomField{
			{ID: TimeSpentLastUpdateFieldID, Value: input.ElapsedSeconds},
			{ID: TimeSpentTotalFieldID, Value: input.ExistingTotalSeconds + input.ElapsedSeconds},
		}
	}
	return req
}

func ExistingTotalSeconds(ticket types.Ticket) int {
	for _, field := range ticket.CustomFields {
		if field.ID != TimeSpentTotalFieldID {
			continue
		}
		switch value := field.Value.(type) {
		case int:
			return value
		case int64:
			return int(value)
		case float64:
			return int(value)
		}
	}
	return 0
}
