package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/itsolver/zentui/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMe(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/users/me", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"user":{"id":123,"name":"Test User","email":"test@example.com","role":"admin","active":true}}`))
	})

	client := testClient(t, handler)
	svc := NewUserService(client)

	user, err := svc.GetMe(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(123), user.ID)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "test@example.com", user.Email)
}

func TestAutocompleteUsers(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/users/autocomplete", r.URL.Path)
		assert.Equal(t, "sarah", r.URL.Query().Get("name"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"users":[{"id":101,"name":"Sarah Chen","email":"sarah@example.com","role":"agent","active":true}]}`))
	})

	client := testClient(t, handler)
	svc := NewUserService(client)

	users, err := svc.AutocompleteUsers(context.Background(), "sarah")
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, "Sarah Chen", users[0].Name)
}

func TestAutocompleteUsers_Empty(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"users":[]}`))
	})

	client := testClient(t, handler)
	svc := NewUserService(client)

	users, err := svc.AutocompleteUsers(context.Background(), "nobody")
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestGetMe_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":"Unauthorized"}`))
	})

	client := testClient(t, handler)
	svc := NewUserService(client)

	_, err := svc.GetMe(context.Background())
	require.Error(t, err)
}

func TestGetUser(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/users/456", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"user":{"id":456,"name":"Requester","email":"requester@example.com","role":"end-user","active":true}}`))
	})

	client := testClient(t, handler)
	svc := NewUserService(client)

	user, err := svc.Get(context.Background(), 456)
	require.NoError(t, err)
	assert.Equal(t, int64(456), user.ID)
	assert.Equal(t, "Requester", user.Name)
}

func TestListIdentities(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/users/456/identities", r.URL.Path)
		assert.Equal(t, "50", r.URL.Query().Get("page[size]"))
		assert.Equal(t, []string{"email", "phone_number"}, r.URL.Query()["type[]"])
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"identities":[{"id":1,"user_id":456,"type":"phone_number","value":"+61400111222","primary":false,"verified":true}],"meta":{"has_more":false}}`))
	})

	client := testClient(t, handler)
	svc := NewUserService(client)

	page, err := svc.ListIdentities(context.Background(), 456, &types.ListUserIdentitiesOptions{
		Limit: 50,
		Types: []string{"email", "phone_number"},
	})
	require.NoError(t, err)
	require.Len(t, page.Identities, 1)
	assert.Equal(t, "phone_number", page.Identities[0].Type)
}

func TestCreateIdentity(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v2/users/456/identities", r.URL.Path)

		var body struct {
			Identity types.CreateUserIdentityRequest `json:"identity"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "phone_number", body.Identity.Type)
		assert.Equal(t, "+61400111222", body.Identity.Value)
		require.NotNil(t, body.Identity.Verified)
		assert.True(t, *body.Identity.Verified)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write([]byte(`{"identity":{"id":2,"user_id":456,"type":"phone_number","value":"+61400111222","primary":false,"verified":true}}`))
	})

	client := testClient(t, handler)
	svc := NewUserService(client)
	verified := true

	identity, err := svc.CreateIdentity(context.Background(), 456, &types.CreateUserIdentityRequest{
		Type:     "phone_number",
		Value:    "+61400111222",
		Verified: &verified,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), identity.ID)
}

func TestMergeEndUser(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/api/v2/users/111/merge", r.URL.Path)

		var body struct {
			User struct {
				ID int64 `json:"id"`
			} `json:"user"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, int64(222), body.User.ID)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"job_status":{"id":"job-1","status":"completed","progress":1,"total":1}}`))
	})

	client := testClient(t, handler)
	svc := NewUserService(client)

	status, err := svc.MergeEndUser(context.Background(), 111, 222)
	require.NoError(t, err)
	assert.Equal(t, "job-1", status.ID)
	assert.Equal(t, "completed", status.Status)
}
