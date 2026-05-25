package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/itsolver/zentui/internal/types"
)

type UserService struct {
	client *Client
}

func NewUserService(client *Client) *UserService {
	return &UserService{client: client}
}

func (s *UserService) GetMe(ctx context.Context) (*types.User, error) {
	var result struct {
		User types.User `json:"user"`
	}
	if err := s.client.doJSON(ctx, "GET", "/api/v2/users/me", nil, &result); err != nil {
		return nil, err
	}
	return &result.User, nil
}

func (s *UserService) Get(ctx context.Context, id int64) (*types.User, error) {
	var result struct {
		User types.User `json:"user"`
	}
	path := fmt.Sprintf("/api/v2/users/%d", id)
	if err := s.client.doJSON(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result.User, nil
}

func (s *UserService) AutocompleteUsers(ctx context.Context, name string) ([]types.User, error) {
	var result struct {
		Users []types.User `json:"users"`
	}
	path := "/api/v2/users/autocomplete?name=" + url.QueryEscape(name)
	if err := s.client.doJSON(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result.Users, nil
}

func (s *UserService) ListIdentities(ctx context.Context, userID int64, opts *types.ListUserIdentitiesOptions) (*types.UserIdentityPage, error) {
	path := fmt.Sprintf("/api/v2/users/%d/identities", userID)
	params := url.Values{}

	if opts != nil {
		if opts.Limit > 0 {
			params.Set("page[size]", strconv.Itoa(opts.Limit))
		}
		if opts.Cursor != "" {
			params.Set("page[after]", opts.Cursor)
		}
		for _, typ := range opts.Types {
			if typ != "" {
				params.Add("type[]", typ)
			}
		}
	}

	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var page types.UserIdentityPage
	if err := s.client.doJSON(ctx, "GET", path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (s *UserService) CreateIdentity(ctx context.Context, userID int64, req *types.CreateUserIdentityRequest) (*types.UserIdentity, error) {
	body := struct {
		Identity *types.CreateUserIdentityRequest `json:"identity"`
	}{Identity: req}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	var result struct {
		Identity types.UserIdentity `json:"identity"`
	}
	path := fmt.Sprintf("/api/v2/users/%d/identities", userID)
	if err := s.client.doJSON(ctx, "POST", path, bytes.NewReader(b), &result); err != nil {
		return nil, err
	}
	return &result.Identity, nil
}

func (s *UserService) MergeEndUser(ctx context.Context, sourceUserID int64, targetUserID int64) (*types.JobStatus, error) {
	body := struct {
		User struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}{}
	body.User.ID = targetUserID

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	var result struct {
		JobStatus types.JobStatus `json:"job_status"`
	}
	path := fmt.Sprintf("/api/v2/users/%d/merge", sourceUserID)
	if err := s.client.doJSON(ctx, "PUT", path, bytes.NewReader(b), &result); err != nil {
		return nil, err
	}
	return &result.JobStatus, nil
}
