package triage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDraftPromptPackUsesCustomerSupportHelper(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "fake-helper")
	require.NoError(t, os.WriteFile(helper, []byte(`#!/bin/sh
cat >/dev/null
echo '{"status":"success","kind":"draft","ticket_id":"123","mode":"public","schema":{"type":"object"},"prompt":"draft prompt"}'
`), 0o700))

	pack, err := BuildDraftPromptPack(context.Background(), dir, helper, 123, "public", nil)

	require.NoError(t, err)
	assert.Equal(t, "success", pack.Status)
	assert.Equal(t, "draft prompt", pack.Prompt)
	assert.JSONEq(t, `{"type":"object"}`, string(pack.Schema))
}

func TestBuildDraftPromptPackRequiresCustomerSupportDir(t *testing.T) {
	_, err := BuildDraftPromptPack(context.Background(), "", "", 123, "public", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "customer-support directory is required")
}

func TestBuildDraftPromptPackIncludesHelperStderr(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "fake-helper")
	require.NoError(t, os.WriteFile(helper, []byte(`#!/bin/sh
echo "missing Zendesk credentials" >&2
exit 1
`), 0o700))

	_, err := BuildDraftPromptPack(context.Background(), dir, helper, 123, "public", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "building draft prompt pack")
	assert.Contains(t, err.Error(), "missing Zendesk credentials")
}

func TestBuildDraftPromptPackPassesHelperEnv(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "fake-helper")
	require.NoError(t, os.WriteFile(helper, []byte(`#!/bin/sh
cat >/dev/null
test "$ZENDESK_SUBDOMAIN" = "itsolver" || exit 2
echo '{"status":"success","kind":"draft","ticket_id":"123","mode":"public","schema":{"type":"object"},"prompt":"draft prompt"}'
`), 0o700))

	pack, err := BuildDraftPromptPack(context.Background(), dir, helper, 123, "public", nil, "ZENDESK_SUBDOMAIN=itsolver")

	require.NoError(t, err)
	assert.Equal(t, "draft prompt", pack.Prompt)
}

func TestNormalizeDraftPromptPackResultUsesHelper(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "fake-helper")
	require.NoError(t, os.WriteFile(helper, []byte(`#!/bin/sh
cat >/dev/null
echo '{"answer":"Clean","recommended_status":"pending","reasoning_summary":"ok"}'
`), 0o700))

	out, err := NormalizeDraftPromptPackResult(context.Background(), dir, helper, "public", DraftOutput{Answer: "Dirty"})

	require.NoError(t, err)
	assert.Equal(t, "Clean", out.Answer)
	assert.Equal(t, "pending", out.RecommendedStatus)
}

func TestBuildImagePromptPackUsesHelper(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "fake-helper")
	require.NoError(t, os.WriteFile(helper, []byte(`#!/bin/sh
echo '{"status":"success","kind":"image","ticket_id":"123","schema":{"type":"object"},"prompt":"image prompt"}'
`), 0o700))

	pack, err := BuildImagePromptPack(context.Background(), dir, helper, 123, "screen.png", "https://example.test/screen.png", "")

	require.NoError(t, err)
	assert.Equal(t, "image", pack.Kind)
	assert.Equal(t, "image prompt", pack.Prompt)
}

func TestBuildAndNormalizeMergePromptPackUsesHelpers(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "fake-helper")
	require.NoError(t, os.WriteFile(helper, []byte(`#!/bin/sh
case "$2" in
  merge-pool)
    echo '{"status":"success","source_ticket":{"id":123},"candidates":[{"id":456,"subject":"Target","status":"open"}]}'
    ;;
  merge-pack)
    cat >/dev/null
    echo '{"status":"success","kind":"merge","ticket_id":"123","schema":{"type":"object"},"prompt":"merge prompt"}'
    ;;
  normalize-merge)
    cat >/dev/null
    echo '{"suggestions":[{"id":456,"subject":"Target","status":"open","relevance_score":91,"rationale":"same issue"}],"recommended_target_id":456}'
    ;;
esac
`), 0o700))

	pool, err := BuildMergePool(context.Background(), dir, helper, 123)
	require.NoError(t, err)
	require.Len(t, pool.Candidates, 1)

	pack, err := BuildMergePromptPack(context.Background(), dir, helper, pool.SourceTicket, pool.Candidates)
	require.NoError(t, err)
	assert.Equal(t, "merge prompt", pack.Prompt)

	normalized, err := NormalizeMergePromptPackResult(context.Background(), dir, helper, []byte(`{"ranked_candidates":[]}`), pool.Candidates)
	require.NoError(t, err)
	require.Len(t, normalized.Suggestions, 1)
	assert.Equal(t, int64(456), normalized.RecommendedTargetID)
}
