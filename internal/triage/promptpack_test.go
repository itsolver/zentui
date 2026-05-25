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
