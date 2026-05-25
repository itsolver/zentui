package demo

import (
	"context"
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
