package codexrunner

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUsageUsesLastTurnCompletedUsage(t *testing.T) {
	stdout := "\n" +
		`{"type":"turn.started"}` + "\n" +
		`{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":3,"reasoning_output_tokens":4}}` + "\n" +
		`not json` + "\n" +
		`{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":70}}`

	usage := ParseUsage(stdout)

	require.NotNil(t, usage)
	assert.Equal(t, 100, usage.InputTokens)
	assert.Equal(t, 20, usage.CachedInputTokens)
	assert.Equal(t, 30, usage.OutputTokens)
	assert.Equal(t, 70, usage.ReasoningOutputTokens)
}

func TestDecodeOutput(t *testing.T) {
	type draft struct {
		Answer            string `json:"answer"`
		RecommendedStatus string `json:"recommended_status"`
	}

	out, err := DecodeOutput[draft](json.RawMessage(`{"answer":"Thanks","recommended_status":"pending"}`))

	require.NoError(t, err)
	assert.Equal(t, "Thanks", out.Answer)
	assert.Equal(t, "pending", out.RecommendedStatus)
}

func TestDecodeOutputRejectsEmpty(t *testing.T) {
	_, err := DecodeOutput[map[string]string](nil)

	require.Error(t, err)
}
