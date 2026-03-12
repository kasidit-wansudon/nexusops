package member

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTeam(t *testing.T) {
	m := NewManager()
	team, err := m.CreateTeam("backend", "Backend team", "owner1")
	require.NoError(t, err)
	assert.NotEmpty(t, team.ID)
	assert.Equal(t, "backend", team.Name)
	assert.Len(t, team.Members, 1)
	assert.Equal(t, "owner", team.Members[0].Role)
	assert.Equal(t, "owner1", team.Members[0].UserID)
}

func TestCreateTeamEmptyName(t *testing.T) {
	m := NewManager()
	_, err := m.CreateTeam("", "desc", "owner1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

func TestCreateTeamEmptyOwner(t *testing.T) {
	m := NewManager()
	_, err := m.CreateTeam("team", "desc", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "owner ID cannot be empty")
}

func TestCreateTeamDuplicateName(t *testing.T) {
	m := NewManager()
	m.CreateTeam("backend", "desc", "owner1")
	_, err := m.CreateTeam("backend", "desc2", "owner2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestGetTeam(t *testing.T) {
	m := NewManager()
	created, _ := m.CreateTeam("team1", "desc", "owner1")
	team, err := m.GetTeam(created.ID)
	require.NoError(t, err)
	assert.Equal(t, "team1", team.Name)
}

func TestGetTeamNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetTeam("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestAddMember(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")

	member, err := m.AddMember(team.ID, "user2", "developer", "owner1")
	require.NoError(t, err)
	assert.Equal(t, "developer", member.Role)
	assert.Equal(t, "owner1", member.InvitedBy)

	members, _ := m.ListMembers(team.ID)
	assert.Len(t, members, 2)
}

func TestAddMemberInvalidRole(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")

	_, err := m.AddMember(team.ID, "user2", "superadmin", "owner1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")
}

func TestAddMemberDuplicate(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")

	_, err := m.AddMember(team.ID, "owner1", "developer", "owner1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already a member")
}

func TestAddMemberEmptyFields(t *testing.T) {
	m := NewManager()
	_, err := m.AddMember("", "user", "developer", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "team ID cannot be empty")

	team, _ := m.CreateTeam("team1", "desc", "owner1")
	_, err = m.AddMember(team.ID, "", "developer", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user ID cannot be empty")
}

func TestRemoveMember(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")
	member, _ := m.AddMember(team.ID, "user2", "developer", "owner1")

	err := m.RemoveMember(team.ID, member.ID)
	require.NoError(t, err)

	members, _ := m.ListMembers(team.ID)
	assert.Len(t, members, 1) // only owner remains
}

func TestRemoveLastOwner(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")
	ownerID := team.Members[0].ID

	err := m.RemoveMember(team.ID, ownerID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "last owner")
}

func TestRemoveMemberNotFound(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")

	err := m.RemoveMember(team.ID, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateRole(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")
	member, _ := m.AddMember(team.ID, "user2", "developer", "owner1")

	err := m.UpdateRole(team.ID, member.ID, "admin")
	require.NoError(t, err)

	members, _ := m.ListMembers(team.ID)
	for _, mem := range members {
		if mem.ID == member.ID {
			assert.Equal(t, "admin", mem.Role)
		}
	}
}

func TestUpdateRoleInvalidRole(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")

	err := m.UpdateRole(team.ID, team.Members[0].ID, "superadmin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")
}

func TestUpdateRoleDemoteLastOwner(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")
	ownerID := team.Members[0].ID

	err := m.UpdateRole(team.ID, ownerID, "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "last owner")
}

func TestListTeams(t *testing.T) {
	m := NewManager()
	m.CreateTeam("team1", "desc", "user1")
	m.CreateTeam("team2", "desc", "user1")
	m.CreateTeam("team3", "desc", "user2")

	teams, err := m.ListTeams("user1")
	require.NoError(t, err)
	assert.Len(t, teams, 2)
}

func TestGetMemberPermissions(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")

	perms := m.GetMemberPermissions(team.ID, "owner1")
	assert.Contains(t, perms, "team.manage")
	assert.Contains(t, perms, "project.read")
}

func TestGetMemberPermissionsViewer(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")
	m.AddMember(team.ID, "viewer1", "viewer", "owner1")

	perms := m.GetMemberPermissions(team.ID, "viewer1")
	assert.Contains(t, perms, "project.read")
	assert.Contains(t, perms, "env.read")
	assert.NotContains(t, perms, "project.write")
}

func TestGetMemberPermissionsNonMember(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("team1", "desc", "owner1")

	perms := m.GetMemberPermissions(team.ID, "stranger")
	assert.Nil(t, perms)
}
