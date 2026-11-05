package access

import (
	"errors"
	"testing"
)

func TestAuthorizeRequiresExplicitGrant(t *testing.T) {
	authorizer, err := NewAuthorizer(map[Role][]Capability{
		RoleModerator: {"chat.moderate"},
	})
	if err != nil {
		t.Fatal(err)
	}

	moderator := Principal{ID: "mod-1", Role: RoleModerator}
	if err := authorizer.Authorize(moderator, "chat.moderate"); err != nil {
		t.Fatalf("explicit grant was denied: %v", err)
	}
	if authorizer.Allows(Principal{ID: "player-1", Role: RolePlayer}, "chat.moderate") {
		t.Fatal("capability was granted to a role without an explicit grant")
	}
	if authorizer.Allows(moderator, "account.delete") {
		t.Fatal("unknown capability was allowed")
	}
	if err := authorizer.Authorize(moderator, "account.delete"); !errors.Is(err, ErrUnknownCapability) {
		t.Fatalf("expected ErrUnknownCapability, got %v", err)
	}
}

func TestAuthorizeRejectsUnknownRole(t *testing.T) {
	authorizer, err := NewAuthorizer(nil)
	if err != nil {
		t.Fatal(err)
	}
	err = authorizer.Authorize(Principal{ID: "unknown-1", Role: "owner"}, "chat.moderate")
	if !errors.Is(err, ErrUnknownRole) {
		t.Fatalf("expected ErrUnknownRole, got %v", err)
	}
}

func TestAssignRoleRequiresGrantAndPreventsEscalation(t *testing.T) {
	authorizer, err := NewAuthorizer(map[Role][]Capability{
		RoleModerator: {CapabilityAssignRole},
	})
	if err != nil {
		t.Fatal(err)
	}
	moderator := Principal{ID: "mod-1", Role: RoleModerator}

	assigned, err := authorizer.AssignRole(moderator, "player-1", RolePlayer)
	if err != nil {
		t.Fatalf("expected lower-rank assignment to succeed: %v", err)
	}
	if assigned != (Principal{ID: "player-1", Role: RolePlayer}) {
		t.Fatalf("unexpected assigned principal: %#v", assigned)
	}

	for _, targetRole := range []Role{RoleModerator, RoleAdmin} {
		if _, err := authorizer.AssignRole(moderator, "target-1", targetRole); !errors.Is(err, ErrRoleEscalation) {
			t.Errorf("assigning %q: expected ErrRoleEscalation, got %v", targetRole, err)
		}
	}
}

func TestAssignRoleDeniedWithoutExplicitGrant(t *testing.T) {
	authorizer, err := NewAuthorizer(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = authorizer.AssignRole(Principal{ID: "admin-1", Role: RoleAdmin}, "player-1", RolePlayer)
	if !errors.Is(err, ErrUnknownCapability) {
		t.Fatalf("expected default-deny error for unconfigured role assignment, got %v", err)
	}
}

func TestNewAuthorizerRejectsUnknownRoleGrant(t *testing.T) {
	_, err := NewAuthorizer(map[Role][]Capability{"owner": {"chat.moderate"}})
	if !errors.Is(err, ErrUnknownRole) {
		t.Fatalf("expected ErrUnknownRole, got %v", err)
	}
}
