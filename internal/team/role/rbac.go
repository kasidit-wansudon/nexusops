package role

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// Permission represents a granular access control permission.
type Permission string

// Permission constants for the NexusOps platform.
const (
	ProjectRead        Permission = "project.read"
	ProjectWrite       Permission = "project.write"
	ProjectDelete      Permission = "project.delete"
	PipelineRun        Permission = "pipeline.run"
	PipelineConfigure  Permission = "pipeline.configure"
	DeployCreate       Permission = "deploy.create"
	DeployRollback     Permission = "deploy.rollback"
	EnvRead            Permission = "env.read"
	EnvWrite           Permission = "env.write"
	TeamManage         Permission = "team.manage"
	SettingsManage     Permission = "settings.manage"
)

// AllPermissions is the complete set of permissions available.
var AllPermissions = []Permission{
	ProjectRead, ProjectWrite, ProjectDelete,
	PipelineRun, PipelineConfigure,
	DeployCreate, DeployRollback,
	EnvRead, EnvWrite,
	TeamManage, SettingsManage,
}

// Role defines a named set of permissions that can be assigned to users.
type Role struct {
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
	Description string       `json:"description"`
	Custom      bool         `json:"custom"`
}

// HasPermission checks whether the role includes the specified permission.
func (r *Role) HasPermission(perm Permission) bool {
	for _, p := range r.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// DefaultRoles returns the built-in role definitions.
func DefaultRoles() map[string]*Role {
	return map[string]*Role{
		"owner": {
			Name: "owner",
			Permissions: []Permission{
				ProjectRead, ProjectWrite, ProjectDelete,
				PipelineRun, PipelineConfigure,
				DeployCreate, DeployRollback,
				EnvRead, EnvWrite,
				TeamManage, SettingsManage,
			},
			Description: "Full access to all resources and team management",
			Custom:      false,
		},
		"admin": {
			Name: "admin",
			Permissions: []Permission{
				ProjectRead, ProjectWrite, ProjectDelete,
				PipelineRun, PipelineConfigure,
				DeployCreate, DeployRollback,
				EnvRead, EnvWrite,
				SettingsManage,
			},
			Description: "Full access to resources except team management",
			Custom:      false,
		},
		"developer": {
			Name: "developer",
			Permissions: []Permission{
				ProjectRead, ProjectWrite,
				PipelineRun,
				DeployCreate,
				EnvRead,
			},
			Description: "Read/write projects, run pipelines, and create deployments",
			Custom:      false,
		},
		"viewer": {
			Name: "viewer",
			Permissions: []Permission{
				ProjectRead,
				EnvRead,
			},
			Description: "Read-only access to projects and environments",
			Custom:      false,
		},
	}
}

// userRoleKey is a composite key for user-project role assignments.
type userRoleKey struct {
	UserID    string
	ProjectID string
}

// RBAC manages role assignments and permission checks for users across projects.
type RBAC struct {
	roles     map[string]*Role
	userRoles map[userRoleKey]string
	mu        sync.RWMutex
}

// NewRBAC creates a new RBAC instance pre-loaded with default roles.
func NewRBAC() *RBAC {
	return &RBAC{
		roles:     DefaultRoles(),
		userRoles: make(map[userRoleKey]string),
	}
}

// AssignRole assigns a named role to a user for a specific project.
func (rb *RBAC) AssignRole(userID, projectID, roleName string) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if _, exists := rb.roles[roleName]; !exists {
		return fmt.Errorf("role %q does not exist", roleName)
	}
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if projectID == "" {
		return fmt.Errorf("project ID cannot be empty")
	}

	key := userRoleKey{UserID: userID, ProjectID: projectID}
	rb.userRoles[key] = roleName
	return nil
}

// RemoveRole removes a user's role assignment for a specific project.
func (rb *RBAC) RemoveRole(userID, projectID string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	key := userRoleKey{UserID: userID, ProjectID: projectID}
	delete(rb.userRoles, key)
}

// HasPermission checks if a user has a specific permission for a project.
func (rb *RBAC) HasPermission(userID, projectID string, perm Permission) bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	key := userRoleKey{UserID: userID, ProjectID: projectID}
	roleName, exists := rb.userRoles[key]
	if !exists {
		return false
	}

	role, exists := rb.roles[roleName]
	if !exists {
		return false
	}

	return role.HasPermission(perm)
}

// GetUserRole returns the role assigned to a user for a given project, or nil
// if no assignment exists.
func (rb *RBAC) GetUserRole(userID, projectID string) *Role {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	key := userRoleKey{UserID: userID, ProjectID: projectID}
	roleName, exists := rb.userRoles[key]
	if !exists {
		return nil
	}

	role, exists := rb.roles[roleName]
	if !exists {
		return nil
	}
	return role
}

// GetUserPermissions returns all permissions a user has for a given project.
func (rb *RBAC) GetUserPermissions(userID, projectID string) []Permission {
	role := rb.GetUserRole(userID, projectID)
	if role == nil {
		return nil
	}
	perms := make([]Permission, len(role.Permissions))
	copy(perms, role.Permissions)
	return perms
}

// CreateCustomRole registers a new custom role with the given permissions.
func (rb *RBAC) CreateCustomRole(name, description string, perms []Permission) (*Role, error) {
	if name == "" {
		return nil, fmt.Errorf("role name cannot be empty")
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	if _, exists := rb.roles[name]; exists {
		return nil, fmt.Errorf("role %q already exists", name)
	}

	// Validate that all permissions are known.
	validPerms := make(map[Permission]bool)
	for _, p := range AllPermissions {
		validPerms[p] = true
	}
	for _, p := range perms {
		if !validPerms[p] {
			return nil, fmt.Errorf("unknown permission %q", p)
		}
	}

	role := &Role{
		Name:        name,
		Permissions: make([]Permission, len(perms)),
		Description: description,
		Custom:      true,
	}
	copy(role.Permissions, perms)
	rb.roles[name] = role

	return role, nil
}

// DeleteCustomRole removes a custom role. Built-in roles cannot be deleted.
func (rb *RBAC) DeleteCustomRole(name string) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	role, exists := rb.roles[name]
	if !exists {
		return fmt.Errorf("role %q does not exist", name)
	}
	if !role.Custom {
		return fmt.Errorf("cannot delete built-in role %q", name)
	}

	delete(rb.roles, name)
	return nil
}

// ListRoles returns all registered roles (both built-in and custom).
func (rb *RBAC) ListRoles() []*Role {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	roles := make([]*Role, 0, len(rb.roles))
	for _, r := range rb.roles {
		roles = append(roles, r)
	}
	return roles
}

// RequirePermission returns a Gin middleware handler that checks whether the
// authenticated user has the specified permission for the project identified in
// the request. It expects "user_id" and "project_id" to be present in the Gin
// context (set by an upstream auth middleware).
func RequirePermission(rbac *RBAC, perm Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}

		projectID := c.Param("project_id")
		if projectID == "" {
			projectID = c.GetString("project_id")
		}
		if projectID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "project_id is required",
			})
			return
		}

		if !rbac.HasPermission(userID, projectID, perm) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":      "insufficient permissions",
				"required":   string(perm),
				"user_id":    userID,
				"project_id": projectID,
			})
			return
		}

		c.Next()
	}
}
