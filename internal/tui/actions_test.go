package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/itsolver/zentui/internal/types"
)

func TestSubmitMergeRunsRequesterCleanupAfterTicketMerge(t *testing.T) {
	var calls []string
	tickets := &mergeOrderTicketService{calls: &calls}
	users := &mergeOrderUserService{calls: &calls}
	model := newActionsModel(tickets, users)
	model.sourceTicketID = 1
	model.mergeCleanupEnabled = true
	model.textarea.SetValue("2")

	msg := model.submitMerge()()

	_, ok := msg.(ticketUpdatedMsg)
	require.True(t, ok, "expected ticketUpdatedMsg, got %T", msg)
	assert.Equal(t, []string{"ticket_merge", "user_merge", "identity_create"}, calls)
}

func TestSubmitMergeReturnsTicketUpdateWhenRequesterCleanupFails(t *testing.T) {
	var calls []string
	tickets := &mergeOrderTicketService{calls: &calls}
	users := &mergeOrderUserService{calls: &calls, mergeErr: errors.New("permission denied")}
	model := newActionsModel(tickets, users)
	model.sourceTicketID = 1
	model.mergeCleanupEnabled = true
	model.textarea.SetValue("2")

	msg := model.submitMerge()()

	updated, ok := msg.(ticketUpdatedMsg)
	require.True(t, ok, "expected ticketUpdatedMsg, got %T", msg)
	require.NotNil(t, updated.warning)
	assert.Contains(t, updated.warning.Error(), "requester cleanup failed after ticket merge")
	assert.Equal(t, int64(2), updated.ticket.ID)
	assert.Equal(t, []string{"ticket_merge", "user_merge"}, calls)
}

type mergeOrderTicketService struct {
	calls *[]string
}

func (s *mergeOrderTicketService) List(context.Context, *types.ListTicketsOptions) (*types.TicketPage, error) {
	return nil, nil
}

func (s *mergeOrderTicketService) ListView(context.Context, int64, *types.ListTicketsOptions) (*types.TicketPage, error) {
	return nil, nil
}

func (s *mergeOrderTicketService) Get(_ context.Context, id int64, _ *types.GetTicketOptions) (*types.TicketResult, error) {
	if id == 1 {
		return &types.TicketResult{
			Ticket: types.Ticket{
				ID:          1,
				Subject:     "Incoming call",
				Description: "Caller phone 0439 651 141",
				Status:      "open",
				Tags:        []string{"zoom_phone"},
				RequesterID: 10,
			},
			Users: []types.User{{ID: 10, Name: "Caller"}},
		}, nil
	}
	return &types.TicketResult{
		Ticket: types.Ticket{
			ID:          2,
			Subject:     "Target ticket",
			Status:      "open",
			RequesterID: 20,
		},
		Users: []types.User{{ID: 20, Name: "Customer"}},
	}, nil
}

func (s *mergeOrderTicketService) Create(context.Context, *types.CreateTicketRequest) (*types.Ticket, error) {
	return nil, nil
}

func (s *mergeOrderTicketService) Update(context.Context, int64, *types.UpdateTicketRequest) (*types.Ticket, error) {
	return nil, nil
}

func (s *mergeOrderTicketService) Delete(context.Context, int64) error {
	return nil
}

func (s *mergeOrderTicketService) ListComments(context.Context, int64, *types.ListCommentsOptions) (*types.CommentPage, error) {
	return nil, nil
}

func (s *mergeOrderTicketService) ListAudits(context.Context, int64, *types.ListAuditsOptions) (*types.AuditPage, error) {
	return &types.AuditPage{}, nil
}

func (s *mergeOrderTicketService) ListTicketFields(context.Context, *types.ListTicketFieldsOptions) (*types.TicketFieldPage, error) {
	return nil, nil
}

func (s *mergeOrderTicketService) MergeTickets(context.Context, int64, *types.MergeTicketsRequest) (*types.MergeTicketsResult, error) {
	*s.calls = append(*s.calls, "ticket_merge")
	return &types.MergeTicketsResult{Ticket: &types.Ticket{ID: 2, Status: "open"}}, nil
}

type mergeOrderUserService struct {
	calls    *[]string
	mergeErr error
}

func (s *mergeOrderUserService) GetMe(context.Context) (*types.User, error) { return nil, nil }
func (s *mergeOrderUserService) Get(context.Context, int64) (*types.User, error) {
	return nil, nil
}
func (s *mergeOrderUserService) AutocompleteUsers(context.Context, string) ([]types.User, error) {
	return nil, nil
}
func (s *mergeOrderUserService) ListIdentities(context.Context, int64, *types.ListUserIdentitiesOptions) (*types.UserIdentityPage, error) {
	return &types.UserIdentityPage{}, nil
}
func (s *mergeOrderUserService) CreateIdentity(context.Context, int64, *types.CreateUserIdentityRequest) (*types.UserIdentity, error) {
	*s.calls = append(*s.calls, "identity_create")
	return &types.UserIdentity{}, nil
}
func (s *mergeOrderUserService) MergeEndUser(context.Context, int64, int64) (*types.JobStatus, error) {
	*s.calls = append(*s.calls, "user_merge")
	if s.mergeErr != nil {
		return nil, s.mergeErr
	}
	return &types.JobStatus{Status: "completed"}, nil
}
