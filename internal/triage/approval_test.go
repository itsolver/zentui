package triage

import (
	"testing"

	"github.com/itsolver/zentui/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildApprovalUpdateIncludesCommentStatusSafeUpdateAndTimeFields(t *testing.T) {
	req := BuildApprovalUpdate(ApprovalInput{
		Body:                 "Thanks, this is fixed.",
		Public:               true,
		ConfirmedStatus:      "solved",
		ElapsedSeconds:       120,
		ExistingTotalSeconds: 300,
		UpdatedStamp:         "2026-05-26T01:23:45Z",
	})

	require.NotNil(t, req.Comment)
	assert.Equal(t, "Thanks, this is fixed.", req.Comment.Body)
	require.NotNil(t, req.Comment.Public)
	assert.True(t, *req.Comment.Public)
	assert.Equal(t, "solved", req.Status)
	assert.True(t, req.SafeUpdate)
	assert.Equal(t, "2026-05-26T01:23:45Z", req.UpdatedStamp)
	require.Len(t, req.CustomFields, 2)
	assert.Equal(t, TimeSpentLastUpdateFieldID, req.CustomFields[0].ID)
	assert.Equal(t, 120, req.CustomFields[0].Value)
	assert.Equal(t, TimeSpentTotalFieldID, req.CustomFields[1].ID)
	assert.Equal(t, 420, req.CustomFields[1].Value)
}

func TestBuildApprovalUpdateOmitsTimeFieldsWhenElapsedZero(t *testing.T) {
	req := BuildApprovalUpdate(ApprovalInput{Body: "Internal note", Public: false})

	require.NotNil(t, req.Comment)
	require.NotNil(t, req.Comment.Public)
	assert.False(t, *req.Comment.Public)
	assert.Empty(t, req.CustomFields)
	assert.False(t, req.SafeUpdate)
	assert.Empty(t, req.UpdatedStamp)
}

func TestExistingTotalSeconds(t *testing.T) {
	ticket := types.Ticket{CustomFields: []types.CustomField{
		{ID: 1, Value: 999},
		{ID: TimeSpentTotalFieldID, Value: float64(123)},
	}}

	assert.Equal(t, 123, ExistingTotalSeconds(ticket))
}
