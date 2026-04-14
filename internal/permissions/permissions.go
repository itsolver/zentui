package permissions

import "github.com/johanviberg/zd/internal/types"

const (
	roleTypeLight = 1
)

// Permissions describes what the authenticated user is allowed to do.
type Permissions struct {
	CanPublicComment bool
	CanChangeStatus  bool
	CanAssignTickets bool
	CanDeleteTickets bool
	CanAddCC         bool
	IsLightAgent     bool
	Role             string // "admin", "agent", "light_agent"
}

// FromUser derives permissions from a Zendesk User.
// Returns full permissions when user is nil (fail-open — the Zendesk API
// enforces restrictions server-side regardless).
func FromUser(u *types.User) Permissions {
	if u == nil {
		return fullPermissions("agent")
	}

	switch u.Role {
	case "admin":
		return fullPermissions("admin")
	case "agent":
		if u.RoleType != nil && *u.RoleType == roleTypeLight {
			return lightAgentPermissions()
		}
		if u.RestrictedAgent {
			return lightAgentPermissions()
		}
		return fullPermissions("agent")
	default:
		return fullPermissions(u.Role)
	}
}

func fullPermissions(role string) Permissions {
	return Permissions{
		CanPublicComment: true,
		CanChangeStatus:  true,
		CanAssignTickets: true,
		CanDeleteTickets: true,
		CanAddCC:         true,
		Role:             role,
	}
}

func lightAgentPermissions() Permissions {
	return Permissions{
		IsLightAgent: true,
		Role:         "light_agent",
	}
}
