package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditEventUnmarshalAcceptsStringBody(t *testing.T) {
	var event AuditEvent

	err := json.Unmarshal([]byte(`{"id":1,"type":"Comment","body":"hello"}`), &event)

	require.NoError(t, err)
	assert.Equal(t, "hello", event.Body)
}

func TestAuditEventUnmarshalAcceptsObjectBody(t *testing.T) {
	var page AuditPage
	raw := []byte(`{
		"audits": [{
			"id": 1,
			"events": [{
				"id": 2,
				"type": "Comment",
				"body": {"message": "structured", "parts": ["a", "b"]}
			}]
		}]
	}`)

	err := json.Unmarshal(raw, &page)

	require.NoError(t, err)
	require.Len(t, page.Audits, 1)
	require.Len(t, page.Audits[0].Events, 1)
	assert.JSONEq(t, `{"message":"structured","parts":["a","b"]}`, page.Audits[0].Events[0].Body)
}
