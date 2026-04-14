package permissions

import (
	"testing"

	"github.com/johanviberg/zd/internal/types"
)

func intPtr(v int) *int { return &v }

func TestFromUser_NilUser(t *testing.T) {
	p := FromUser(nil)
	if p.IsLightAgent {
		t.Error("nil user should not be light agent")
	}
	if !p.CanPublicComment || !p.CanChangeStatus || !p.CanAssignTickets || !p.CanDeleteTickets || !p.CanAddCC {
		t.Error("nil user should have full permissions (fail-open)")
	}
}

func TestFromUser_Admin(t *testing.T) {
	u := &types.User{Role: "admin"}
	p := FromUser(u)
	if p.IsLightAgent {
		t.Error("admin should not be light agent")
	}
	if p.Role != "admin" {
		t.Errorf("expected role admin, got %s", p.Role)
	}
	if !p.CanPublicComment || !p.CanChangeStatus || !p.CanDeleteTickets {
		t.Error("admin should have full permissions")
	}
}

func TestFromUser_RegularAgent(t *testing.T) {
	u := &types.User{Role: "agent"}
	p := FromUser(u)
	if p.IsLightAgent {
		t.Error("regular agent should not be light agent")
	}
	if p.Role != "agent" {
		t.Errorf("expected role agent, got %s", p.Role)
	}
	if !p.CanPublicComment || !p.CanChangeStatus || !p.CanDeleteTickets || !p.CanAssignTickets || !p.CanAddCC {
		t.Error("regular agent should have full permissions")
	}
}

func TestFromUser_LightAgent_ByRoleType(t *testing.T) {
	u := &types.User{Role: "agent", RoleType: intPtr(1)}
	p := FromUser(u)
	if !p.IsLightAgent {
		t.Error("should be identified as light agent")
	}
	if p.Role != "light_agent" {
		t.Errorf("expected role light_agent, got %s", p.Role)
	}
	if p.CanPublicComment || p.CanChangeStatus || p.CanAssignTickets || p.CanDeleteTickets || p.CanAddCC {
		t.Error("light agent should have restricted permissions")
	}
}

func TestFromUser_LightAgent_ByRestrictedFlag(t *testing.T) {
	u := &types.User{Role: "agent", RestrictedAgent: true}
	p := FromUser(u)
	if !p.IsLightAgent {
		t.Error("restricted agent should be treated as light agent")
	}
	if p.CanPublicComment {
		t.Error("restricted agent should not be able to post public comments")
	}
}

func TestFromUser_CustomAgent(t *testing.T) {
	// role_type=0 is a custom agent role, not a light agent
	u := &types.User{Role: "agent", RoleType: intPtr(0)}
	p := FromUser(u)
	if p.IsLightAgent {
		t.Error("custom agent (role_type=0) should not be light agent")
	}
	if !p.CanPublicComment {
		t.Error("custom agent should have full permissions")
	}
}

func TestFromUser_EndUser(t *testing.T) {
	u := &types.User{Role: "end-user"}
	p := FromUser(u)
	if p.IsLightAgent {
		t.Error("end-user should not be light agent")
	}
	// fail-open: end-users get full permissions from our side; API enforces
	if !p.CanPublicComment {
		t.Error("end-user should get full permissions (fail-open)")
	}
}
