package member

import (
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.teams == nil {
		t.Error("teams map should be initialized")
	}
	if m.members == nil {
		t.Error("members map should be initialized")
	}
}

func TestCreateTeam(t *testing.T) {
	tests := []struct {
		name        string
		teamName    string
		description string
		ownerID     string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid team creation",
			teamName:    "backend",
			description: "Backend team",
			ownerID:     "user-1",
			wantErr:     false,
		},
		{
			name:        "empty team name",
			teamName:    "",
			description: "No name",
			ownerID:     "user-1",
			wantErr:     true,
			errContains: "team name cannot be empty",
		},
		{
			name:        "empty owner ID",
			teamName:    "frontend",
			description: "Frontend team",
			ownerID:     "",
			wantErr:     true,
			errContains: "owner ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			team, err := m.CreateTeam(tt.teamName, tt.description, tt.ownerID)

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
			if team == nil {
				t.Fatal("expected team but got nil")
			}
			if team.Name != tt.teamName {
				t.Errorf("team name = %q, want %q", team.Name, tt.teamName)
			}
			if team.ID == "" {
				t.Error("team ID should not be empty")
			}
			if team.CreatedAt.IsZero() {
				t.Error("team CreatedAt should not be zero")
			}
			if len(team.Members) != 1 {
				t.Fatalf("expected 1 member (owner), got %d", len(team.Members))
			}
			if team.Members[0].Role != "owner" {
				t.Errorf("first member role = %q, want %q", team.Members[0].Role, "owner")
			}
			if team.Members[0].UserID != tt.ownerID {
				t.Errorf("owner user ID = %q, want %q", team.Members[0].UserID, tt.ownerID)
			}
		})
	}
}

func TestCreateTeamDuplicateName(t *testing.T) {
	m := NewManager()
	_, err := m.CreateTeam("alpha", "first", "user-1")
	if err != nil {
		t.Fatalf("unexpected error creating first team: %v", err)
	}

	_, err = m.CreateTeam("alpha", "second", "user-2")
	if err == nil {
		t.Fatal("expected error for duplicate team name but got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error %q should mention 'already exists'", err.Error())
	}
}

func TestAddMember(t *testing.T) {
	m := NewManager()
	team, err := m.CreateTeam("backend", "Backend", "owner-1")
	if err != nil {
		t.Fatalf("failed to create team: %v", err)
	}

	tests := []struct {
		name        string
		teamID      string
		userID      string
		role        string
		invitedBy   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "add developer",
			teamID:    team.ID,
			userID:    "user-2",
			role:      "developer",
			invitedBy: "owner-1",
			wantErr:   false,
		},
		{
			name:        "empty team ID",
			teamID:      "",
			userID:      "user-3",
			role:        "viewer",
			invitedBy:   "owner-1",
			wantErr:     true,
			errContains: "team ID cannot be empty",
		},
		{
			name:        "empty user ID",
			teamID:      team.ID,
			userID:      "",
			role:        "viewer",
			invitedBy:   "owner-1",
			wantErr:     true,
			errContains: "user ID cannot be empty",
		},
		{
			name:        "invalid role",
			teamID:      team.ID,
			userID:      "user-4",
			role:        "superadmin",
			invitedBy:   "owner-1",
			wantErr:     true,
			errContains: "invalid role",
		},
		{
			name:        "nonexistent team",
			teamID:      "no-such-team",
			userID:      "user-5",
			role:        "viewer",
			invitedBy:   "owner-1",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "duplicate membership",
			teamID:      team.ID,
			userID:      "owner-1",
			role:        "admin",
			invitedBy:   "owner-1",
			wantErr:     true,
			errContains: "already a member",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			member, err := m.AddMember(tt.teamID, tt.userID, tt.role, tt.invitedBy)

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
			if member == nil {
				t.Fatal("expected member but got nil")
			}
			if member.UserID != tt.userID {
				t.Errorf("member UserID = %q, want %q", member.UserID, tt.userID)
			}
			if member.Role != tt.role {
				t.Errorf("member Role = %q, want %q", member.Role, tt.role)
			}
			if member.TeamID != tt.teamID {
				t.Errorf("member TeamID = %q, want %q", member.TeamID, tt.teamID)
			}
			if member.InvitedBy != tt.invitedBy {
				t.Errorf("member InvitedBy = %q, want %q", member.InvitedBy, tt.invitedBy)
			}
			if member.JoinedAt.IsZero() {
				t.Error("member JoinedAt should not be zero")
			}
		})
	}
}

func TestRemoveMember(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("backend", "Backend", "owner-1")
	dev, _ := m.AddMember(team.ID, "user-2", "developer", "owner-1")

	// Remove the developer -- should succeed.
	err := m.RemoveMember(team.ID, dev.ID)
	if err != nil {
		t.Fatalf("unexpected error removing developer: %v", err)
	}

	// Verify member is gone.
	members, _ := m.ListMembers(team.ID)
	for _, mem := range members {
		if mem.ID == dev.ID {
			t.Error("removed member should not appear in ListMembers")
		}
	}
	if len(members) != 1 {
		t.Errorf("expected 1 member remaining, got %d", len(members))
	}

	// Try to remove the last owner -- should fail.
	ownerID := team.Members[0].ID
	err = m.RemoveMember(team.ID, ownerID)
	if err == nil {
		t.Fatal("expected error when removing last owner but got nil")
	}
	if !strings.Contains(err.Error(), "last owner") {
		t.Errorf("error %q should mention 'last owner'", err.Error())
	}

	// Try to remove from nonexistent team.
	err = m.RemoveMember("no-such-team", dev.ID)
	if err == nil {
		t.Fatal("expected error for nonexistent team but got nil")
	}

	// Try to remove nonexistent member.
	err = m.RemoveMember(team.ID, "no-such-member")
	if err == nil {
		t.Fatal("expected error for nonexistent member but got nil")
	}
}

func TestRemoveOwnerWhenMultipleExist(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("backend", "Backend", "owner-1")

	// Add a second owner.
	_, err := m.AddMember(team.ID, "owner-2", "owner", "owner-1")
	if err != nil {
		t.Fatalf("unexpected error adding second owner: %v", err)
	}

	// Now removing the first owner should succeed because another owner exists.
	err = m.RemoveMember(team.ID, team.Members[0].ID)
	if err != nil {
		t.Fatalf("expected no error when removing one of multiple owners, got: %v", err)
	}
}

func TestListMembers(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("backend", "Backend", "owner-1")
	m.AddMember(team.ID, "user-2", "developer", "owner-1")
	m.AddMember(team.ID, "user-3", "viewer", "owner-1")

	members, err := m.ListMembers(team.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("expected 3 members, got %d", len(members))
	}

	userIDs := make(map[string]bool)
	for _, mem := range members {
		userIDs[mem.UserID] = true
	}
	for _, uid := range []string{"owner-1", "user-2", "user-3"} {
		if !userIDs[uid] {
			t.Errorf("expected user %q in member list", uid)
		}
	}

	// Nonexistent team should return error.
	_, err = m.ListMembers("no-such-team")
	if err == nil {
		t.Fatal("expected error for nonexistent team but got nil")
	}
}

func TestUpdateRole(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("backend", "Backend", "owner-1")
	dev, _ := m.AddMember(team.ID, "user-2", "developer", "owner-1")

	// Promote developer to admin.
	err := m.UpdateRole(team.ID, dev.ID, "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	members, _ := m.ListMembers(team.ID)
	for _, mem := range members {
		if mem.ID == dev.ID {
			if mem.Role != "admin" {
				t.Errorf("role after update = %q, want %q", mem.Role, "admin")
			}
		}
	}

	// Invalid role should fail.
	err = m.UpdateRole(team.ID, dev.ID, "superuser")
	if err == nil {
		t.Fatal("expected error for invalid role but got nil")
	}
	if !strings.Contains(err.Error(), "invalid role") {
		t.Errorf("error %q should contain 'invalid role'", err.Error())
	}

	// Demoting the last owner should fail.
	ownerMemberID := team.Members[0].ID
	err = m.UpdateRole(team.ID, ownerMemberID, "viewer")
	if err == nil {
		t.Fatal("expected error when demoting last owner but got nil")
	}
	if !strings.Contains(err.Error(), "last owner") {
		t.Errorf("error %q should mention 'last owner'", err.Error())
	}

	// Nonexistent team should fail.
	err = m.UpdateRole("no-such-team", dev.ID, "viewer")
	if err == nil {
		t.Fatal("expected error for nonexistent team but got nil")
	}

	// Nonexistent member should fail.
	err = m.UpdateRole(team.ID, "no-such-member", "viewer")
	if err == nil {
		t.Fatal("expected error for nonexistent member but got nil")
	}
}

func TestGetMemberPermissions(t *testing.T) {
	m := NewManager()
	team, _ := m.CreateTeam("backend", "Backend", "owner-1")
	m.AddMember(team.ID, "user-2", "viewer", "owner-1")

	// Owner should have team.manage permission.
	ownerPerms := m.GetMemberPermissions(team.ID, "owner-1")
	if ownerPerms == nil {
		t.Fatal("expected permissions for owner but got nil")
	}
	found := false
	for _, p := range ownerPerms {
		if p == "team.manage" {
			found = true
			break
		}
	}
	if !found {
		t.Error("owner should have 'team.manage' permission")
	}

	// Viewer should have project.read but not project.write.
	viewerPerms := m.GetMemberPermissions(team.ID, "user-2")
	if viewerPerms == nil {
		t.Fatal("expected permissions for viewer but got nil")
	}
	hasRead := false
	hasWrite := false
	for _, p := range viewerPerms {
		if p == "project.read" {
			hasRead = true
		}
		if p == "project.write" {
			hasWrite = true
		}
	}
	if !hasRead {
		t.Error("viewer should have 'project.read' permission")
	}
	if hasWrite {
		t.Error("viewer should not have 'project.write' permission")
	}

	// Non-member should get nil.
	nonePerms := m.GetMemberPermissions(team.ID, "non-existent-user")
	if nonePerms != nil {
		t.Errorf("expected nil permissions for non-member, got %v", nonePerms)
	}

	// Nonexistent team should get nil.
	noTeamPerms := m.GetMemberPermissions("no-such-team", "owner-1")
	if noTeamPerms != nil {
		t.Errorf("expected nil permissions for nonexistent team, got %v", noTeamPerms)
	}
}
