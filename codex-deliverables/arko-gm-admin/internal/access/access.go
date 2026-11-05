package access

import (
	"errors"
	"fmt"
	"strings"
)

type Role string

const (
	RolePlayer    Role = "player"
	RoleModerator Role = "moderator"
	RoleAdmin     Role = "admin"
)

type Capability string

const CapabilityAssignRole Capability = "role.assign"

var roleRanks = map[Role]int{
	RolePlayer:    1,
	RoleModerator: 2,
	RoleAdmin:     3,
}

var (
	ErrUnknownRole       = errors.New("unknown role")
	ErrUnknownCapability = errors.New("unknown capability")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrRoleEscalation    = errors.New("role assignment would equal or exceed actor rank")
	ErrInvalidPrincipal  = errors.New("invalid principal")
	ErrInvalidGrant      = errors.New("invalid capability grant")
)

type AuthorizationError struct {
	Operation   string
	ActorRole   Role
	TargetRole  Role
	PrincipalID string
	Capability  Capability
	Err         error
}

func (e *AuthorizationError) Error() string {
	switch {
	case e.Capability != "":
		return fmt.Sprintf("%s: capability %q for role %q: %v", e.Operation, e.Capability, e.ActorRole, e.Err)
	case e.TargetRole != "":
		return fmt.Sprintf("%s: actor role %q cannot assign role %q: %v", e.Operation, e.ActorRole, e.TargetRole, e.Err)
	case e.ActorRole != "":
		return fmt.Sprintf("%s: role %q: %v", e.Operation, e.ActorRole, e.Err)
	default:
		return fmt.Sprintf("%s: principal %q: %v", e.Operation, e.PrincipalID, e.Err)
	}
}

func (e *AuthorizationError) Unwrap() error {
	return e.Err
}

type Principal struct {
	ID   string
	Role Role
}

type Authorizer struct {
	grants       map[Role]map[Capability]struct{}
	capabilities map[Capability]struct{}
}

// NewAuthorizer copies the supplied grants. A capability is recognized only
// when it appears in at least one configured role grant.
func NewAuthorizer(grants map[Role][]Capability) (*Authorizer, error) {
	a := &Authorizer{
		grants:       make(map[Role]map[Capability]struct{}, len(grants)),
		capabilities: make(map[Capability]struct{}),
	}
	for role, capabilities := range grants {
		if _, ok := roleRanks[role]; !ok {
			return nil, &AuthorizationError{Operation: "configure grants", ActorRole: role, Err: ErrUnknownRole}
		}
		a.grants[role] = make(map[Capability]struct{}, len(capabilities))
		for _, capability := range capabilities {
			if capability == "" || strings.TrimSpace(string(capability)) != string(capability) {
				return nil, &AuthorizationError{Operation: "configure grants", ActorRole: role, Capability: capability, Err: ErrInvalidGrant}
			}
			a.grants[role][capability] = struct{}{}
			a.capabilities[capability] = struct{}{}
		}
	}
	return a, nil
}

// RoleRank returns the configured hierarchy rank for a built-in role.
func RoleRank(role Role) (int, error) {
	rank, ok := roleRanks[role]
	if !ok {
		return 0, &AuthorizationError{Operation: "read role rank", ActorRole: role, Err: ErrUnknownRole}
	}
	return rank, nil
}

// Authorize reports whether principal has capability. No role has capabilities
// unless the capability is explicitly present in NewAuthorizer's grants.
func (a *Authorizer) Authorize(principal Principal, capability Capability) error {
	if strings.TrimSpace(principal.ID) == "" {
		return &AuthorizationError{Operation: "authorize", PrincipalID: principal.ID, Err: ErrInvalidPrincipal}
	}
	if _, ok := roleRanks[principal.Role]; !ok {
		return &AuthorizationError{Operation: "authorize", ActorRole: principal.Role, Err: ErrUnknownRole}
	}
	if capability == "" || a == nil {
		return &AuthorizationError{Operation: "authorize", ActorRole: principal.Role, Capability: capability, Err: ErrUnknownCapability}
	}
	if _, ok := a.capabilities[capability]; !ok {
		return &AuthorizationError{Operation: "authorize", ActorRole: principal.Role, Capability: capability, Err: ErrUnknownCapability}
	}
	if _, ok := a.grants[principal.Role][capability]; !ok {
		return &AuthorizationError{Operation: "authorize", ActorRole: principal.Role, Capability: capability, Err: ErrPermissionDenied}
	}
	return nil
}

// Allows is the boolean form of Authorize. It denies on every error.
func (a *Authorizer) Allows(principal Principal, capability Capability) bool {
	return a.Authorize(principal, capability) == nil
}

// AssignRole returns a principal with role assigned only when the actor has an
// explicit role.assign grant and the target role ranks strictly below theirs.
func (a *Authorizer) AssignRole(actor Principal, principalID string, role Role) (Principal, error) {
	if strings.TrimSpace(principalID) == "" {
		return Principal{}, &AuthorizationError{Operation: "assign role", PrincipalID: principalID, Err: ErrInvalidPrincipal}
	}
	if _, ok := roleRanks[role]; !ok {
		return Principal{}, &AuthorizationError{Operation: "assign role", ActorRole: actor.Role, TargetRole: role, Err: ErrUnknownRole}
	}
	actorRank, ok := roleRanks[actor.Role]
	if !ok {
		return Principal{}, &AuthorizationError{Operation: "assign role", ActorRole: actor.Role, TargetRole: role, Err: ErrUnknownRole}
	}
	if roleRanks[role] >= actorRank {
		return Principal{}, &AuthorizationError{Operation: "assign role", ActorRole: actor.Role, TargetRole: role, Err: ErrRoleEscalation}
	}
	if err := a.Authorize(actor, CapabilityAssignRole); err != nil {
		return Principal{}, err
	}
	return Principal{ID: principalID, Role: role}, nil
}
