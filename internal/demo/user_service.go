package demo

import (
	"context"
	"fmt"
	"strings"

	"github.com/itsolver/zentui/internal/types"
)

type UserService struct {
	store *Store
}

func NewUserService(store *Store) *UserService {
	return &UserService{store: store}
}

func (s *UserService) GetMe(ctx context.Context) (*types.User, error) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	role := s.store.DemoRole
	for i := range s.store.Users {
		u := &s.store.Users[i]
		switch role {
		case "light_agent":
			if u.RestrictedAgent {
				copy := *u
				return &copy, nil
			}
		case "admin":
			if u.Role == "admin" {
				copy := *u
				return &copy, nil
			}
		default:
			if u.Role == "agent" && !u.RestrictedAgent {
				copy := *u
				return &copy, nil
			}
		}
	}
	return nil, types.NewNotFoundError("no matching user found")
}

func (s *UserService) AutocompleteUsers(ctx context.Context, name string) ([]types.User, error) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	query := strings.ToLower(name)
	var matches []types.User
	for _, u := range s.store.Users {
		if strings.Contains(strings.ToLower(u.Name), query) || strings.Contains(strings.ToLower(u.Email), query) {
			matches = append(matches, u)
		}
	}
	return matches, nil
}

func (s *UserService) Get(ctx context.Context, id int64) (*types.User, error) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	for i := range s.store.Users {
		if s.store.Users[i].ID == id {
			copy := s.store.Users[i]
			return &copy, nil
		}
	}
	return nil, types.NewNotFoundError(fmt.Sprintf("user %d not found", id))
}

func (s *UserService) ListIdentities(ctx context.Context, userID int64, opts *types.ListUserIdentitiesOptions) (*types.UserIdentityPage, error) {
	user, err := s.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	identity := types.UserIdentity{
		ID:       user.ID*100 + 1,
		UserID:   user.ID,
		Type:     "email",
		Value:    user.Email,
		Primary:  true,
		Verified: true,
	}
	return &types.UserIdentityPage{Identities: []types.UserIdentity{identity}}, nil
}

func (s *UserService) CreateIdentity(ctx context.Context, userID int64, req *types.CreateUserIdentityRequest) (*types.UserIdentity, error) {
	if _, err := s.Get(ctx, userID); err != nil {
		return nil, err
	}

	identity := &types.UserIdentity{
		ID:                 userID*100 + 2,
		UserID:             userID,
		Type:               req.Type,
		Value:              req.Value,
		VerificationMethod: req.VerificationMethod,
	}
	if req.Verified != nil {
		identity.Verified = *req.Verified
	}
	if req.Primary != nil {
		identity.Primary = *req.Primary
	}
	return identity, nil
}

func (s *UserService) MergeEndUser(ctx context.Context, sourceUserID int64, targetUserID int64) (*types.JobStatus, error) {
	if _, err := s.Get(ctx, sourceUserID); err != nil {
		return nil, err
	}
	if _, err := s.Get(ctx, targetUserID); err != nil {
		return nil, err
	}
	return &types.JobStatus{ID: "demo", Status: "completed", Total: 1, Progress: 1}, nil
}
