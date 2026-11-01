// Package capbac implements Capability-Based Access Control (CapBAC) and authorization contracts.
package capbac

import (
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// Role defines the allowed agent or user personas bound to CUE #Role.
type Role string

const (
	RoleArchitect Role = "architect"
	RoleDeveloper Role = "developer"
	RoleObserver Role = "observer"
	RoleGuest     Role = "guest"
	RoleCritic    Role = "critic"
)

// IsValid evaluates whether Role matches the CUE #Role definition.
func (r Role) IsValid() bool {
	switch r {
	case RoleArchitect, RoleDeveloper, RoleObserver, RoleGuest, RoleCritic:
		return true
	default:
		return false
	}
}

// Action defines the authorized execution operations bound to CUE #Action.
type Action string

const (
	ActionRead    Action = "read"
	ActionWrite   Action = "write"
	ActionTest    Action = "test"
	ActionVerify  Action = "verify"
	ActionRelease Action = "release"
)

// IsValid evaluates whether Action matches the CUE #Action definition.
func (a Action) IsValid() bool {
	switch a {
	case ActionRead, ActionWrite, ActionTest, ActionVerify, ActionRelease:
		return true
	default:
		return false
	}
}

// regexResource validates that resource contains no whitespace characters.
var regexResource = regexp.MustCompile(`^\S+$`)

// AuthzPolicy defines the authoritative authorization evaluation contract bound to CUE #AuthzPolicy.
type AuthzPolicy struct {
	Role     Role   `json:"role"`
	Action   Action `json:"action"`
	Resource string `json:"resource"`
	Allow    bool   `json:"allow"`
}

// Validate ensures AuthzPolicy satisfies all CUE schema constraints.
func (p *AuthzPolicy) Validate() error {
	if p == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "authz policy cannot be nil")
	}
	if !p.Role.IsValid() {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid role %q: must be architect, developer, observer, guest, or critic", p.Role)
	}
	if !p.Action.IsValid() {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid action %q: must be read, write, test, verify, or release", p.Action)
	}
	if !regexResource.MatchString(p.Resource) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid resource %q: must be non-empty without whitespace", p.Resource)
	}
	return nil
}

// EvaluateAuthz evaluates authorization predicates adhering to CUE #AuthzPolicy (Law 7 Least-Privilege).
func EvaluateAuthz(role Role, action Action, resource string) bool {
	cleanResource := strings.TrimSpace(resource)
	if cleanResource == "" || !regexResource.MatchString(cleanResource) {
		return false
	}

	switch role {
	case RoleArchitect:
		return true
	case RoleDeveloper:
		return action == ActionRead || action == ActionWrite || action == ActionTest
	case RoleObserver, RoleGuest:
		return action == ActionRead
	case RoleCritic:
		return action == ActionRead || action == ActionVerify
	default:
		return false
	}
}
