package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/itsolver/zentui/internal/permissions"
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

func TestApprovalSubmitIncludesUpdatedStampForSafeUpdate(t *testing.T) {
	tickets := &mergeOrderTicketService{}
	model := newActionsModel(tickets, nil)
	updatedAt := time.Date(2026, 5, 26, 1, 23, 45, 0, time.FixedZone("AEST", 10*60*60))
	model, _ = model.openApproval(123, permissions.Permissions{CanPublicComment: true}, "Internal note", "pending", "open", 58, 100, updatedAt, "reason")
	model.isPublic = false
	model.statusIdx = 1

	msg := model.submitApproval()()

	_, ok := msg.(ticketUpdatedMsg)
	require.True(t, ok, "expected ticketUpdatedMsg, got %T", msg)
	require.NotNil(t, tickets.updateReq)
	assert.True(t, tickets.updateReq.SafeUpdate)
	assert.Equal(t, "2026-05-25T15:23:45Z", tickets.updateReq.UpdatedStamp)
	require.NotNil(t, tickets.updateReq.Comment)
	require.NotNil(t, tickets.updateReq.Comment.Public)
	assert.False(t, *tickets.updateReq.Comment.Public)
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
	calls     *[]string
	updateReq *types.UpdateTicketRequest
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

func (s *mergeOrderTicketService) Update(_ context.Context, _ int64, req *types.UpdateTicketRequest) (*types.Ticket, error) {
	s.updateReq = &types.UpdateTicketRequest{}
	*s.updateReq = *req
	return &types.Ticket{ID: 123, Status: req.Status}, nil
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
