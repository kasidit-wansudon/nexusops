package role

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRolesExist(t *testing.T) {
	roles := DefaultRoles()
	assert.Contains(t, roles, "owner")
	assert.Contains(t, roles, "admin")
	assert.Contains(t, roles, "developer")
	assert.Contains(t, roles, "viewer")

	// Owner should have all permissions
	owner := roles["owner"]
	assert.Len(t, owner.Permissions, 11)
	assert.True(t, owner.HasPermission(TeamManage))
	assert.False(t, owner.Custom)

	// Admin should not have TeamManage
	admin := roles["admin"]
	assert.False(t, admin.HasPermission(TeamManage))
	assert.True(t, admin.HasPermission(SettingsManage))

	// Viewer should only have read permissions
	viewer := roles["viewer"]
	assert.True(t, viewer.HasPermission(ProjectRead))
	assert.True(t, viewer.HasPermission(EnvRead))
	assert.False(t, viewer.HasPermission(ProjectWrite))
}

func TestAssignAndCheckPermission(t *testing.T) {
	rbac := NewRBAC()

	err := rbac.AssignRole("user-1", "proj-1", "developer")
	require.NoError(t, err)

	assert.True(t, rbac.HasPermission("user-1", "proj-1", ProjectRead))
	assert.True(t, rbac.HasPermission("user-1", "proj-1", PipelineRun))
	assert.False(t, rbac.HasPermission("user-1", "proj-1", TeamManage))

	// No role assigned
	assert.False(t, rbac.HasPermission("user-2", "proj-1", ProjectRead))
}

func TestAssignRoleValidation(t *testing.T) {
	rbac := NewRBAC()

	tests := []struct {
		name      string
		userID    string
		projectID string
		roleName  string
		errMsg    string
	}{
		{"nonexistent role", "user", "proj", "superhero", "does not exist"},
		{"empty user", "", "proj", "viewer", "user ID cannot be empty"},
		{"empty project", "user", "", "viewer", "project ID cannot be empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := rbac.AssignRole(tc.userID, tc.projectID, tc.roleName)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestRemoveRole(t *testing.T) {
	rbac := NewRBAC()

	_ = rbac.AssignRole("user-1", "proj-1", "owner")
	assert.True(t, rbac.HasPermission("user-1", "proj-1", TeamManage))

	rbac.RemoveRole("user-1", "proj-1")
	assert.False(t, rbac.HasPermission("user-1", "proj-1", TeamManage))
}

func TestGetUserRoleAndPermissions(t *testing.T) {
	rbac := NewRBAC()

	// No assignment
	assert.Nil(t, rbac.GetUserRole("user-1", "proj-1"))
	assert.Nil(t, rbac.GetUserPermissions("user-1", "proj-1"))

	_ = rbac.AssignRole("user-1", "proj-1", "developer")

	role := rbac.GetUserRole("user-1", "proj-1")
	require.NotNil(t, role)
	assert.Equal(t, "developer", role.Name)

	perms := rbac.GetUserPermissions("user-1", "proj-1")
	assert.Len(t, perms, 5)
	assert.Contains(t, perms, ProjectRead)
	assert.Contains(t, perms, PipelineRun)
}

func TestCreateCustomRole(t *testing.T) {
	rbac := NewRBAC()

	role, err := rbac.CreateCustomRole("qa", "QA tester role", []Permission{ProjectRead, PipelineRun})
	require.NoError(t, err)
	assert.Equal(t, "qa", role.Name)
	assert.True(t, role.Custom)
	assert.Len(t, role.Permissions, 2)

	// Assign custom role
	err = rbac.AssignRole("user-1", "proj-1", "qa")
	require.NoError(t, err)
	assert.True(t, rbac.HasPermission("user-1", "proj-1", PipelineRun))
	assert.False(t, rbac.HasPermission("user-1", "proj-1", DeployCreate))

	// Duplicate name
	_, err = rbac.CreateCustomRole("qa", "duplicate", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Empty name
	_, err = rbac.CreateCustomRole("", "no name", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")

	// Invalid permission
	_, err = rbac.CreateCustomRole("bad", "bad perms", []Permission{"fake.perm"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown permission")
}

func TestDeleteCustomRole(t *testing.T) {
	rbac := NewRBAC()

	// Cannot delete built-in role
	err := rbac.DeleteCustomRole("owner")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete built-in")

	// Cannot delete nonexistent
	err = rbac.DeleteCustomRole("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")

	// Create and delete custom role
	_, _ = rbac.CreateCustomRole("temp", "temporary", []Permission{ProjectRead})
	err = rbac.DeleteCustomRole("temp")
	require.NoError(t, err)

	// Verify deleted
	err = rbac.AssignRole("user", "proj", "temp")
	assert.Error(t, err)
}

func TestListRoles(t *testing.T) {
	rbac := NewRBAC()

	roles := rbac.ListRoles()
	assert.Len(t, roles, 4) // 4 default roles

	_, _ = rbac.CreateCustomRole("custom", "desc", []Permission{ProjectRead})
	roles = rbac.ListRoles()
	assert.Len(t, roles, 5)
}
