package capbac

import (
	"testing"
)

func TestRole_IsValid(t *testing.T) {
	tests := []struct {
		role  Role
		valid bool
	}{
		{RoleArchitect, true},
		{RoleDeveloper, true},
		{RoleObserver, true},
		{RoleGuest, true},
		{RoleCritic, true},
		{Role("admin"), false},
		{Role(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.IsValid(); got != tt.valid {
				t.Errorf("Role(%s).IsValid() = %v; want %v", tt.role, got, tt.valid)
			}
		})
	}
}

func TestAction_IsValid(t *testing.T) {
	tests := []struct {
		action Action
		valid  bool
	}{
		{ActionRead, true},
		{ActionWrite, true},
		{ActionTest, true},
		{ActionVerify, true},
		{ActionRelease, true},
		{Action("delete"), false},
		{Action(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.IsValid(); got != tt.valid {
				t.Errorf("Action(%s).IsValid() = %v; want %v", tt.action, got, tt.valid)
			}
		})
	}
}

func TestAuthzPolicy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		policy  *AuthzPolicy
		wantErr bool
	}{
		{
			name: "valid policy",
			policy: &AuthzPolicy{
				Role:     RoleDeveloper,
				Action:   ActionWrite,
				Resource: "substrate/core/workspace",
				Allow:    true,
			},
			wantErr: false,
		},
		{
			name:    "nil policy",
			policy:  nil,
			wantErr: true,
		},
		{
			name: "invalid role",
			policy: &AuthzPolicy{
				Role:     Role("superuser"),
				Action:   ActionRead,
				Resource: "res",
			},
			wantErr: true,
		},
		{
			name: "invalid action",
			policy: &AuthzPolicy{
				Role:     RoleDeveloper,
				Action:   Action("exec"),
				Resource: "res",
			},
			wantErr: true,
		},
		{
			name: "empty resource",
			policy: &AuthzPolicy{
				Role:     RoleDeveloper,
				Action:   ActionRead,
				Resource: "",
			},
			wantErr: true,
		},
		{
			name: "resource with spaces",
			policy: &AuthzPolicy{
				Role:     RoleDeveloper,
				Action:   ActionRead,
				Resource: "path with spaces",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEvaluateAuthz(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		action   Action
		resource string
		expected bool
	}{
		{"architect full access", RoleArchitect, ActionRelease, "workspace", true},
		{"developer read allowed", RoleDeveloper, ActionRead, "workspace", true},
		{"developer write allowed", RoleDeveloper, ActionWrite, "workspace", true},
		{"developer test allowed", RoleDeveloper, ActionTest, "workspace", true},
		{"developer release forbidden", RoleDeveloper, ActionRelease, "workspace", false},
		{"observer read allowed", RoleObserver, ActionRead, "workspace", true},
		{"observer write forbidden", RoleObserver, ActionWrite, "workspace", false},
		{"guest read allowed", RoleGuest, ActionRead, "workspace", true},
		{"guest test forbidden", RoleGuest, ActionTest, "workspace", false},
		{"critic read allowed", RoleCritic, ActionRead, "workspace", true},
		{"critic verify allowed", RoleCritic, ActionVerify, "workspace", true},
		{"critic write forbidden", RoleCritic, ActionWrite, "workspace", false},
		{"empty resource forbidden", RoleArchitect, ActionRead, "", false},
		{"resource with space forbidden", RoleArchitect, ActionRead, "my res", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateAuthz(tt.role, tt.action, tt.resource)
			if got != tt.expected {
				t.Errorf("EvaluateAuthz(%s, %s, %s) = %v; want %v", tt.role, tt.action, tt.resource, got, tt.expected)
			}
		})
	}
}
