package triage

import (
	"context"
	"errors"
	"testing"

	"github.com/itsolver/zentui/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequesterCleanupPlanEligibleForZoomPhoneDifferentRequester(t *testing.T) {
	sourceUser := &types.User{ID: 10, Name: "Caller"}
	targetUser := &types.User{ID: 20, Name: "Customer"}
	plan := BuildRequesterCleanupPlan(
		types.Ticket{ID: 1, Subject: "Incoming call from Rod", Description: "Inbound phone call with Rod (0439 651 141).", Tags: []string{"zoom_phone"}, RequesterID: 10},
		nil,
		sourceUser,
		types.Ticket{ID: 2, Subject: "Target", RequesterID: 20},
		targetUser,
	)

	assert.True(t, plan.Eligible)
	assert.True(t, plan.DefaultEnabled)
	assert.Equal(t, "+61439651141", plan.PhoneNumber)
	assert.Equal(t, int64(10), plan.SourceUser.ID)
	assert.Equal(t, int64(20), plan.TargetUser.ID)
}

func TestBuildRequesterCleanupPlanBlocksAmbiguousPhone(t *testing.T) {
	plan := BuildRequesterCleanupPlan(
		types.Ticket{ID: 1, Subject: "Incoming call", Description: "Call 0439 651 141 or 0400 111 222", Tags: []string{"zoom_phone"}, RequesterID: 10},
		nil,
		&types.User{ID: 10},
		types.Ticket{ID: 2, RequesterID: 20},
		&types.User{ID: 20},
	)

	assert.False(t, plan.Eligible)
	assert.Equal(t, "ambiguous_phone_number", plan.Reason)
}

func TestExecuteRequesterCleanupSkipsUnavailablePlan(t *testing.T) {
	result, err := ExecuteRequesterCleanup(context.Background(), &cleanupUserService{}, RequesterCleanupPlan{Eligible: false, Reason: "missing_phone_number"})

	require.NoError(t, err)
	assert.Equal(t, "skipped", result.Status)
	assert.Equal(t, "missing_phone_number", result.Reason)
}

func TestExecuteRequesterCleanupMergesUserAndCreatesPhoneIdentity(t *testing.T) {
	svc := &cleanupUserService{}
	result, err := ExecuteRequesterCleanup(context.Background(), svc, RequesterCleanupPlan{
		Eligible:    true,
		PhoneNumber: "+61439651141",
		SourceUser:  UserSummary{ID: 10},
		TargetUser:  UserSummary{ID: 20},
	})

	require.NoError(t, err)
	assert.Equal(t, "complete", result.Status)
	assert.Equal(t, int64(10), svc.mergedSource)
	assert.Equal(t, int64(20), svc.mergedTarget)
	assert.Equal(t, "+61439651141", svc.createdPhone)
}

func TestExecuteRequesterCleanupFailsBeforeTicketMergeOnRequesterMergeError(t *testing.T) {
	svc := &cleanupUserService{mergeErr: errors.New("boom")}
	result, err := ExecuteRequesterCleanup(context.Background(), svc, RequesterCleanupPlan{
		Eligible:    true,
		PhoneNumber: "+61439651141",
		SourceUser:  UserSummary{ID: 10},
		TargetUser:  UserSummary{ID: 20},
	})

	require.Error(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Equal(t, "requester_merge_failed", result.Reason)
	assert.Empty(t, svc.createdPhone)
}

type cleanupUserService struct {
	mergedSource int64
	mergedTarget int64
	createdPhone string
	mergeErr     error
}

func (s *cleanupUserService) GetMe(context.Context) (*types.User, error)      { return nil, nil }
func (s *cleanupUserService) Get(context.Context, int64) (*types.User, error) { return nil, nil }
func (s *cleanupUserService) AutocompleteUsers(context.Context, string) ([]types.User, error) {
	return nil, nil
}
func (s *cleanupUserService) ListIdentities(context.Context, int64, *types.ListUserIdentitiesOptions) (*types.UserIdentityPage, error) {
	return &types.UserIdentityPage{}, nil
}
func (s *cleanupUserService) CreateIdentity(_ context.Context, _ int64, req *types.CreateUserIdentityRequest) (*types.UserIdentity, error) {
	s.createdPhone = req.Value
	return &types.UserIdentity{Type: req.Type, Value: req.Value}, nil
}
func (s *cleanupUserService) MergeEndUser(_ context.Context, sourceUserID int64, targetUserID int64) (*types.JobStatus, error) {
	if s.mergeErr != nil {
		return nil, s.mergeErr
	}
	s.mergedSource = sourceUserID
	s.mergedTarget = targetUserID
	return &types.JobStatus{Status: "completed"}, nil
}
