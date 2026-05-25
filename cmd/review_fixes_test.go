package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/itsolver/zentui/internal/demo"
)

func TestDeleteConfirmationPersistsAcrossInvocations(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	require.NoError(t, saveDeleteConfirmation("confirm-123", 12345, now))
	path := filepath.Join(tmp, "zentui", "delete-confirmations.json")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	require.NoError(t, consumeDeleteConfirmation("confirm-123", 12345, now.Add(time.Minute)))

	err = consumeDeleteConfirmation("confirm-123", 12345, now.Add(2*time.Minute))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired")
}

func TestDeleteConfirmationRejectsWrongTicket(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	require.NoError(t, saveDeleteConfirmation("confirm-123", 12345, now))
	err := consumeDeleteConfirmation("confirm-123", 999, now.Add(time.Minute))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "was for ticket 12345")
}

func TestDeleteConfirmationExpires(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	require.NoError(t, saveDeleteConfirmation("confirm-123", 12345, now))
	err := consumeDeleteConfirmation("confirm-123", 12345, now.Add(deleteConfirmationTTL+time.Second))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired")
}

func TestGenerateSchemaIntegerDefaultIsNumeric(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "example",
		Short: "Example command",
	}
	cmd.Flags().Int64("limit", 20, "Maximum records")

	schema := generateSchema(cmd)
	properties := schema["properties"].(map[string]interface{})
	limit := properties["limit"].(map[string]interface{})

	assert.Equal(t, "integer", limit["type"])
	assert.Equal(t, int64(20), limit["default"])
}

func TestValidateIfExists(t *testing.T) {
	for _, value := range []string{"skip", "update", "error"} {
		require.NoError(t, validateIfExists(value))
	}

	err := validateIfExists("overwrite")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --if-exists")
}

func TestDefaultTriageViewIDUsesEnvOnly(t *testing.T) {
	t.Setenv("TICKET_TRIAGE_VIEW_ID", "")
	assert.Zero(t, defaultTriageViewID())

	t.Setenv("TICKET_TRIAGE_VIEW_ID", "7484423111055")
	assert.Equal(t, int64(7484423111055), defaultTriageViewID())

	t.Setenv("TICKET_TRIAGE_VIEW_ID", "not-a-number")
	assert.Zero(t, defaultTriageViewID())
}

func TestDefaultCustomerSupportDirUsesEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZENTUI_CUSTOMER_SUPPORT_DIR", dir)

	assert.Equal(t, dir, defaultCustomerSupportDir())
}

func TestNewArticleServiceHonorsDemoMode(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("profile", "default", "")
	cmd.Flags().String("trace-id", "", "")
	cmd.SetContext(context.WithValue(context.Background(), ctxKeyDemoStore, demo.NewStore()))

	svc, err := newArticleService(cmd)
	require.NoError(t, err)

	page, err := svc.List(context.Background(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, page.Articles)
}
