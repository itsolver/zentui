package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
