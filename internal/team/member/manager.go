package member

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ValidRoles defines the allowed roles for team members.
var ValidRoles = map[string]bool{
	"owner":     true,
	"admin":     true,
	"developer": true,
	"viewer":    true,
}

// rolePermissions maps each role to a set of permission strings.
var rolePermissions = map[string][]string{
	"owner": {
		"project.read", "project.write", "project.delete",
		"pipeline.run", "pipeline.configure",
		"deploy.create", "deploy.rollback",
		"env.read", "env.write",
		"team.manage", "settings.manage",
	},
	"admin": {
		"project.read", "project.write", "project.delete",
		"pipeline.run", "pipeline.configure",
		"deploy.create", "deploy.rollback",
		"env.read", "env.write",
		"settings.manage",
	},
	"developer": {
		"project.read", "project.write",
		"pipeline.run",
		"deploy.create",
		"env.read",
	},
	"viewer": {
		"project.read",
		"env.read",
	},
}

// Member represents a team member with their role and profile information.
type Member struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TeamID    string    `json:"team_id"`
	Role      string    `json:"role"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url"`
	JoinedAt  time.Time `json:"joined_at"`
	InvitedBy string    `json:"invited_by"`
}

// Team represents a group of members working together.
type Team struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	Members     []*Member `json:"members"`
}

// Manager handles team and member CRUD operations with thread-safe access.
type Manager struct {
	teams   map[string]*Team
	members map[string]*Member
	mu      sync.RWMutex
}

// NewManager creates a new Manager with initialized maps.
func NewManager() *Manager {
	return &Manager{
		teams:   make(map[string]*Team),
		members: make(map[string]*Member),
	}
}

// CreateTeam creates a new team with the given owner as the first member.
func (m *Manager) CreateTeam(name, description, ownerID string) (*Team, error) {
	if name == "" {
		return nil, fmt.Errorf("team name cannot be empty")
	}
	if ownerID == "" {
		return nil, fmt.Errorf("owner ID cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate team names.
	for _, t := range m.teams {
		if t.Name == name {
			return nil, fmt.Errorf("team with name %q already exists", name)
		}
	}

	team := &Team{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		Members:     make([]*Member, 0),
	}

	// Create the owner member.
	ownerMember := &Member{
		ID:        uuid.New().String(),
		UserID:    ownerID,
		TeamID:    team.ID,
		Role:      "owner",
		Name:      "",
		Email:     "",
		AvatarURL: "",
		JoinedAt:  time.Now(),
		InvitedBy: "",
	}

	team.Members = append(team.Members, ownerMember)
	m.teams[team.ID] = team
	m.members[ownerMember.ID] = ownerMember

	return team, nil
}

// GetTeam retrieves a team by its ID.
func (m *Manager) GetTeam(teamID string) (*Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	team, exists := m.teams[teamID]
	if !exists {
		return nil, fmt.Errorf("team %q not found", teamID)
	}
	return team, nil
}

// AddMember adds a new member to a team after validating the role and checking
// for duplicate membership.
func (m *Manager) AddMember(teamID, userID, role, invitedBy string) (*Member, error) {
	if teamID == "" {
		return nil, fmt.Errorf("team ID cannot be empty")
	}
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}
	if !ValidRoles[role] {
		return nil, fmt.Errorf("invalid role %q: must be one of owner, admin, developer, viewer", role)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	team, exists := m.teams[teamID]
	if !exists {
		return nil, fmt.Errorf("team %q not found", teamID)
	}

	// Check for duplicate membership.
	for _, mem := range team.Members {
		if mem.UserID == userID {
			return nil, fmt.Errorf("user %q is already a member of team %q", userID, teamID)
		}
	}

	member := &Member{
		ID:        uuid.New().String(),
		UserID:    userID,
		TeamID:    teamID,
		Role:      role,
		JoinedAt:  time.Now(),
		InvitedBy: invitedBy,
	}

	team.Members = append(team.Members, member)
	m.members[member.ID] = member

	return member, nil
}

// RemoveMember removes a member from a team. It prevents removing the last
// owner to ensure every team retains at least one owner.
func (m *Manager) RemoveMember(teamID, memberID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, exists := m.teams[teamID]
	if !exists {
		return fmt.Errorf("team %q not found", teamID)
	}

	var target *Member
	targetIdx := -1
	for i, mem := range team.Members {
		if mem.ID == memberID {
			target = mem
			targetIdx = i
			break
		}
	}
	if target == nil {
		return fmt.Errorf("member %q not found in team %q", memberID, teamID)
	}

	// Prevent removing the last owner.
	if target.Role == "owner" {
		ownerCount := 0
		for _, mem := range team.Members {
			if mem.Role == "owner" {
				ownerCount++
			}
		}
		if ownerCount <= 1 {
			return fmt.Errorf("cannot remove the last owner of team %q", teamID)
		}
	}

	// Remove from team members slice.
	team.Members = append(team.Members[:targetIdx], team.Members[targetIdx+1:]...)
	delete(m.members, memberID)

	return nil
}

// UpdateRole changes a member's role within a team. It prevents demoting the
// last owner so the team always has at least one.
func (m *Manager) UpdateRole(teamID, memberID, newRole string) error {
	if !ValidRoles[newRole] {
		return fmt.Errorf("invalid role %q: must be one of owner, admin, developer, viewer", newRole)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	team, exists := m.teams[teamID]
	if !exists {
		return fmt.Errorf("team %q not found", teamID)
	}

	var target *Member
	for _, mem := range team.Members {
		if mem.ID == memberID {
			target = mem
			break
		}
	}
	if target == nil {
		return fmt.Errorf("member %q not found in team %q", memberID, teamID)
	}

	// Prevent demoting the last owner.
	if target.Role == "owner" && newRole != "owner" {
		ownerCount := 0
		for _, mem := range team.Members {
			if mem.Role == "owner" {
				ownerCount++
			}
		}
		if ownerCount <= 1 {
			return fmt.Errorf("cannot change role of the last owner in team %q", teamID)
		}
	}

	target.Role = newRole
	return nil
}

// ListMembers returns all members of a given team.
func (m *Manager) ListMembers(teamID string) ([]*Member, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	team, exists := m.teams[teamID]
	if !exists {
		return nil, fmt.Errorf("team %q not found", teamID)
	}

	result := make([]*Member, len(team.Members))
	copy(result, team.Members)
	return result, nil
}

// ListTeams returns all teams that a given user belongs to.
func (m *Manager) ListTeams(userID string) ([]*Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var teams []*Team
	for _, team := range m.teams {
		for _, mem := range team.Members {
			if mem.UserID == userID {
				teams = append(teams, team)
				break
			}
		}
	}
	return teams, nil
}

// GetMemberPermissions returns the list of permissions derived from the
// member's role in the specified team. Returns nil if the user is not a member.
func (m *Manager) GetMemberPermissions(teamID, userID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	team, exists := m.teams[teamID]
	if !exists {
		return nil
	}

	for _, mem := range team.Members {
		if mem.UserID == userID {
			perms, ok := rolePermissions[mem.Role]
			if !ok {
				return nil
			}
			result := make([]string, len(perms))
			copy(result, perms)
			return result
		}
	}
	return nil
}
