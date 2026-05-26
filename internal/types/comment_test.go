package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommentUnmarshalMetadataViaSource(t *testing.T) {
	var page CommentPage
	raw := []byte(`{
		"comments": [{
			"id": 1,
			"audit_id": 101,
			"body": "Body",
			"plain_body": "Plain",
			"metadata": {
				"via": {
					"channel": "email",
					"source": {
						"from": {"name": "Sender", "address": "sender@example.com"},
						"to": {"name": "Support", "address": "support@example.com"}
					}
				}
			}
		}]
	}`)

	err := json.Unmarshal(raw, &page)

	require.NoError(t, err)
	require.Len(t, page.Comments, 1)
	comment := page.Comments[0]
	assert.Equal(t, int64(101), comment.AuditID)
	assert.Equal(t, "Plain", comment.PlainBody)
	require.NotNil(t, comment.Metadata)
	require.NotNil(t, comment.Metadata.Via)
	assert.Equal(t, "email", comment.Metadata.Via.Channel)
	require.Len(t, comment.Metadata.Via.Source.From, 1)
	require.Len(t, comment.Metadata.Via.Source.To, 1)
	assert.Equal(t, "Sender", comment.Metadata.Via.Source.From[0].Name)
	assert.Equal(t, "support@example.com", comment.Metadata.Via.Source.To[0].Address)
}

func TestCommentViaPartiesAcceptArrays(t *testing.T) {
	var comment Comment
	raw := []byte(`{
		"id": 1,
		"body": "Body",
		"metadata": {
			"via": {
				"source": {
					"to": [
						{"name": "First", "address": "first@example.com"},
						{"name": "Second", "address": "second@example.com"}
					]
				}
			}
		}
	}`)

	err := json.Unmarshal(raw, &comment)

	require.NoError(t, err)
	require.NotNil(t, comment.Metadata)
	require.NotNil(t, comment.Metadata.Via)
	require.Len(t, comment.Metadata.Via.Source.To, 2)
	assert.Equal(t, "Second", comment.Metadata.Via.Source.To[1].Name)
}
