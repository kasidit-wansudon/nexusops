package role

import (
	"strings"
	"testing"
)

func TestNewRBAC(t *testing.T) {
	rbac := NewRBAC()
	if rbac == nil {
		t.Fatal("NewRBAC returned nil")
	}

	// Should have the 4 default roles.
	roles := rbac.ListRoles()
	if len(roles) != 4 {
		t.Errorf("expected 4 default roles, got %d", len(roles))
	}

	names := make(map[string]bool)
	for _, r := range roles {
		names[r.Name] = true
	}
	for _, expected := range []string{"owner", "admin", "developer", "viewer"} {
		if !names[expected] {
			t.Errorf("expected default role %q to be present", expected)
		}
	}
}

func TestAssignRole(t *testing.T) {
	tests := []struct {
		name        string
		userID      string
		projectID   string
		roleName    string
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid assignment",
			userID:    "user-1",
			projectID: "proj-1",
			roleName:  "developer",
			wantErr:   false,
		},
		{
			name:        "nonexistent role",
			userID:      "user-1",
			projectID:   "proj-1",
			roleName:    "nonexistent",
			wantErr:     true,
			errContains: "does not exist",
		},
		{
			name:        "empty user ID",
			userID:      "",
			projectID:   "proj-1",
			roleName:    "developer",
			wantErr:     true,
			errContains: "user ID cannot be empty",
		},
		{
			name:        "empty project ID",
			userID:      "user-1",
			projectID:   "",
			roleName:    "developer",
			wantErr:     true,
			errContains: "project ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rbac := NewRBAC()
			err := rbac.AssignRole(tt.userID, tt.projectID, tt.roleName)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify role was assigned.
			role := rbac.GetUserRole(tt.userID, tt.projectID)
			if role == nil {
				t.Fatal("expected role after assignment but got nil")
			}
			if role.Name != tt.roleName {
				t.Errorf("assigned role name = %q, want %q", role.Name, tt.roleName)
			}
		})
	}
}

func TestAssignRoleOverwrite(t *testing.T) {
	rbac := NewRBAC()

	err := rbac.AssignRole("user-1", "proj-1", "viewer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = rbac.AssignRole("user-1", "proj-1", "admin")
	if err != nil {
		t.Fatalf("unexpected error on overwrite: %v", err)
	}

	role := rbac.GetUserRole("user-1", "proj-1")
	if role == nil {
		t.Fatal("expected role after overwrite but got nil")
	}
	if role.Name != "admin" {
		t.Errorf("overwritten role = %q, want %q", role.Name, "admin")
	}
}

func TestHasPermission(t *testing.T) {
	rbac := NewRBAC()

	// No role assigned -- should deny everything.
	if rbac.HasPermission("user-no-role", "proj-1", ProjectRead) {
		t.Error("user with no role should not have ProjectRead")
	}

	// Assign developer role.
	if err := rbac.AssignRole("dev-user", "proj-1", "developer"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Developer should have these permissions.
	allowed := []Permission{ProjectRead, ProjectWrite, PipelineRun, DeployCreate, EnvRead}
	for _, perm := range allowed {
		if !rbac.HasPermission("dev-user", "proj-1", perm) {
			t.Errorf("developer should have permission %s", perm)
		}
	}

	// Developer should NOT have these permissions.
	denied := []Permission{ProjectDelete, PipelineConfigure, DeployRollback, EnvWrite, TeamManage, SettingsManage}
	for _, perm := range denied {
		if rbac.HasPermission("dev-user", "proj-1", perm) {
			t.Errorf("developer should not have permission %s", perm)
		}
	}

	// Different project should deny.
	if rbac.HasPermission("dev-user", "proj-2", ProjectRead) {
		t.Error("permission for a different project should be denied")
	}
}

func TestHasPermissionOwner(t *testing.T) {
	rbac := NewRBAC()

	if err := rbac.AssignRole("owner-user", "proj-1", "owner"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Owner should have ALL permissions.
	for _, perm := range AllPermissions {
		if !rbac.HasPermission("owner-user", "proj-1", perm) {
			t.Errorf("owner should have permission %s", perm)
		}
	}
}

func TestHasPermissionViewer(t *testing.T) {
	rbac := NewRBAC()

	if err := rbac.AssignRole("viewer-user", "proj-1", "viewer"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Viewer should only have read permissions.
	if !rbac.HasPermission("viewer-user", "proj-1", ProjectRead) {
		t.Error("viewer should have ProjectRead")
	}
	if !rbac.HasPermission("viewer-user", "proj-1", EnvRead) {
		t.Error("viewer should have EnvRead")
	}

	deniedPerms := []Permission{
		ProjectWrite, ProjectDelete,
		PipelineRun, PipelineConfigure,
		DeployCreate, DeployRollback,
		EnvWrite, TeamManage, SettingsManage,
	}
	for _, perm := range deniedPerms {
		if rbac.HasPermission("viewer-user", "proj-1", perm) {
			t.Errorf("viewer should not have permission %s", perm)
		}
	}
}

func TestGetUserRoles(t *testing.T) {
	rbac := NewRBAC()

	// No role assigned.
	role := rbac.GetUserRole("user-1", "proj-1")
	if role != nil {
		t.Errorf("expected nil for unassigned user, got %v", role)
	}

	// Assign a role and verify.
	if err := rbac.AssignRole("user-1", "proj-1", "admin"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	role = rbac.GetUserRole("user-1", "proj-1")
	if role == nil {
		t.Fatal("expected role but got nil")
	}
	if role.Name != "admin" {
		t.Errorf("role name = %q, want %q", role.Name, "admin")
	}

	// GetUserPermissions should return admin's permissions.
	perms := rbac.GetUserPermissions("user-1", "proj-1")
	if perms == nil {
		t.Fatal("expected permissions but got nil")
	}
	if len(perms) != 10 {
		t.Errorf("admin should have 10 permissions, got %d", len(perms))
	}

	// No role on different project.
	perms = rbac.GetUserPermissions("user-1", "proj-2")
	if perms != nil {
		t.Errorf("expected nil permissions for unassigned project, got %v", perms)
	}
}

func TestCreateCustomRole(t *testing.T) {
	rbac := NewRBAC()

	customPerms := []Permission{ProjectRead, PipelineRun}
	role, err := rbac.CreateCustomRole("ci-bot", "CI/CD bot with limited access", customPerms)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if role.Name != "ci-bot" {
		t.Errorf("role name = %q, want %q", role.Name, "ci-bot")
	}
	if !role.Custom {
		t.Error("custom role should have Custom = true")
	}
	if len(role.Permissions) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(role.Permissions))
	}

	// Should be assignable and functional.
	if err := rbac.AssignRole("bot-user", "proj-1", "ci-bot"); err != nil {
		t.Fatalf("unexpected error assigning custom role: %v", err)
	}
	if !rbac.HasPermission("bot-user", "proj-1", ProjectRead) {
		t.Error("ci-bot should have ProjectRead")
	}
	if !rbac.HasPermission("bot-user", "proj-1", PipelineRun) {
		t.Error("ci-bot should have PipelineRun")
	}
	if rbac.HasPermission("bot-user", "proj-1", ProjectWrite) {
		t.Error("ci-bot should not have ProjectWrite")
	}

	// Duplicate name should fail.
	_, err = rbac.CreateCustomRole("ci-bot", "duplicate", []Permission{ProjectRead})
	if err == nil {
		t.Fatal("expected error for duplicate role name but got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error %q should mention 'already exists'", err.Error())
	}

	// Empty name should fail.
	_, err = rbac.CreateCustomRole("", "empty name", []Permission{ProjectRead})
	if err == nil {
		t.Fatal("expected error for empty role name but got nil")
	}
}

func TestDeleteCustomRole(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.CreateCustomRole("temp-role", "temporary", []Permission{ProjectRead})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = rbac.DeleteCustomRole("temp-role")
	if err != nil {
		t.Fatalf("unexpected error deleting custom role: %v", err)
	}

	// Should no longer be assignable.
	err = rbac.AssignRole("user-1", "proj-1", "temp-role")
	if err == nil {
		t.Fatal("expected error assigning deleted role but got nil")
	}

	// Deleting a built-in role should fail.
	err = rbac.DeleteCustomRole("admin")
	if err == nil {
		t.Fatal("expected error deleting built-in role but got nil")
	}
	if !strings.Contains(err.Error(), "cannot delete built-in role") {
		t.Errorf("error %q should mention 'cannot delete built-in role'", err.Error())
	}

	// Deleting a nonexistent role should fail.
	err = rbac.DeleteCustomRole("nonexistent")
	if err == nil {
		t.Fatal("expected error deleting nonexistent role but got nil")
	}
}

func TestRoleHasPermission(t *testing.T) {
	r := &Role{
		Name:        "test",
		Permissions: []Permission{ProjectRead, EnvRead},
	}

	if !r.HasPermission(ProjectRead) {
		t.Error("role should have ProjectRead")
	}
	if !r.HasPermission(EnvRead) {
		t.Error("role should have EnvRead")
	}
	if r.HasPermission(ProjectWrite) {
		t.Error("role should not have ProjectWrite")
	}
	if r.HasPermission(TeamManage) {
		t.Error("role should not have TeamManage")
	}
}
