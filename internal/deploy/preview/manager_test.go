package preview

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// --- test doubles ---

type fakeDeployer struct {
	containers map[string]bool
	nextID     int
	deployErr  error
	stopErr    error
	stopped    []string
}

func newFakeDeployer() *fakeDeployer {
	return &fakeDeployer{containers: make(map[string]bool)}
}

func (f *fakeDeployer) DeployPreview(_ context.Context, image string, port int, env map[string]string) (string, int, error) {
	if f.deployErr != nil {
		return "", 0, f.deployErr
	}
	f.nextID++
	id := fmt.Sprintf("container-%d", f.nextID)
	f.containers[id] = true
	return id, port, nil
}

func (f *fakeDeployer) StopPreview(_ context.Context, containerID string) error {
	if f.stopErr != nil {
		return f.stopErr
	}
	delete(f.containers, containerID)
	f.stopped = append(f.stopped, containerID)
	return nil
}

type fakeRouter struct {
	routes  map[string]string
	removed []string
	addErr  error
	rmErr   error
}

func newFakeRouter() *fakeRouter {
	return &fakeRouter{routes: make(map[string]string)}
}

func (f *fakeRouter) AddRoute(subdomain string, backendAddr string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.routes[subdomain] = backendAddr
	return nil
}

func (f *fakeRouter) RemoveRoute(subdomain string) error {
	if f.rmErr != nil {
		return f.rmErr
	}
	delete(f.routes, subdomain)
	f.removed = append(f.removed, subdomain)
	return nil
}

// --- tests ---

func TestNewManager(t *testing.T) {
	deployer := newFakeDeployer()
	router := newFakeRouter()
	mgr := NewManager("preview.example.com", deployer, router)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.domain != "preview.example.com" {
		t.Errorf("domain = %q, want %q", mgr.domain, "preview.example.com")
	}
	if mgr.basePort != 9000 {
		t.Errorf("basePort = %d, want 9000", mgr.basePort)
	}
	if mgr.ttl != 72*time.Hour {
		t.Errorf("ttl = %v, want %v", mgr.ttl, 72*time.Hour)
	}
	if mgr.Count() != 0 {
		t.Errorf("Count() = %d, want 0 for fresh manager", mgr.Count())
	}
}

func TestCreatePreview(t *testing.T) {
	tests := []struct {
		name      string
		projectID string
		prNumber  int
		image     string
		branch    string
		wantErr   string
	}{
		{
			name:      "successful creation",
			projectID: "proj-1",
			prNumber:  10,
			image:     "myimage:latest",
			branch:    "feature-branch",
		},
		{
			name:      "empty project ID",
			projectID: "",
			prNumber:  1,
			image:     "img",
			branch:    "branch",
			wantErr:   "project ID is required",
		},
		{
			name:      "zero PR number",
			projectID: "proj",
			prNumber:  0,
			image:     "img",
			branch:    "branch",
			wantErr:   "PR number must be positive",
		},
		{
			name:      "negative PR number",
			projectID: "proj",
			prNumber:  -5,
			image:     "img",
			branch:    "branch",
			wantErr:   "PR number must be positive",
		},
		{
			name:      "empty image",
			projectID: "proj",
			prNumber:  1,
			image:     "",
			branch:    "branch",
			wantErr:   "image is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deployer := newFakeDeployer()
			router := newFakeRouter()
			mgr := NewManager("preview.example.com", deployer, router)
			ctx := context.Background()

			env, err := mgr.Create(ctx, tc.projectID, tc.prNumber, tc.image, tc.branch)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if env == nil {
				t.Fatal("expected non-nil environment")
			}
			if env.ProjectID != tc.projectID {
				t.Errorf("ProjectID = %q, want %q", env.ProjectID, tc.projectID)
			}
			if env.PRNumber != tc.prNumber {
				t.Errorf("PRNumber = %d, want %d", env.PRNumber, tc.prNumber)
			}
			if env.Branch != tc.branch {
				t.Errorf("Branch = %q, want %q", env.Branch, tc.branch)
			}
			if env.Status != "running" {
				t.Errorf("Status = %q, want %q", env.Status, "running")
			}
			if env.ID == "" {
				t.Error("expected non-empty ID")
			}
			if env.ContainerID == "" {
				t.Error("expected non-empty ContainerID")
			}
			if env.Subdomain == "" {
				t.Error("expected non-empty Subdomain")
			}
			if !strings.Contains(env.URL, "preview.example.com") {
				t.Errorf("URL = %q, want it to contain domain", env.URL)
			}
			if env.CreatedAt.IsZero() {
				t.Error("CreatedAt should not be zero")
			}
			if env.ExpiresAt.IsZero() {
				t.Error("ExpiresAt should not be zero")
			}
			if mgr.Count() != 1 {
				t.Errorf("Count() = %d, want 1", mgr.Count())
			}
		})
	}
}

func TestCreatePreview_DuplicatePR(t *testing.T) {
	deployer := newFakeDeployer()
	router := newFakeRouter()
	mgr := NewManager("preview.example.com", deployer, router)
	ctx := context.Background()

	_, err := mgr.Create(ctx, "proj-1", 10, "myimage:latest", "feature-branch")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = mgr.Create(ctx, "proj-1", 10, "myimage:latest", "feature-branch")
	if err == nil {
		t.Fatal("expected error for duplicate PR, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want substring %q", err.Error(), "already exists")
	}
}

func TestCreatePreview_DeployError(t *testing.T) {
	deployer := newFakeDeployer()
	deployer.deployErr = fmt.Errorf("docker is down")
	router := newFakeRouter()
	mgr := NewManager("preview.example.com", deployer, router)

	env, err := mgr.Create(context.Background(), "proj-1", 1, "img:v1", "main")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "deploy preview container") {
		t.Errorf("error = %q, want deploy error", err.Error())
	}
	if env == nil {
		t.Fatal("expected non-nil environment on error")
	}
	if env.Status != "error" {
		t.Errorf("Status = %q, want %q", env.Status, "error")
	}
}

func TestDeletePreview(t *testing.T) {
	deployer := newFakeDeployer()
	router := newFakeRouter()
	mgr := NewManager("preview.example.com", deployer, router)
	ctx := context.Background()

	env, err := mgr.Create(ctx, "proj-1", 5, "img:v1", "branch")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if mgr.Count() != 1 {
		t.Fatalf("Count() = %d, want 1 after create", mgr.Count())
	}

	err = mgr.Delete(ctx, env.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if mgr.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after delete", mgr.Count())
	}

	// Get should fail after deletion.
	_, err = mgr.Get(env.ID)
	if err == nil {
		t.Error("expected error on Get after delete, got nil")
	}

	// Delete nonexistent env should fail.
	err = mgr.Delete(ctx, "no-such-id")
	if err == nil {
		t.Error("expected error deleting nonexistent env, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want substring %q", err.Error(), "not found")
	}
}

func TestListByProject(t *testing.T) {
	deployer := newFakeDeployer()
	router := newFakeRouter()
	mgr := NewManager("preview.example.com", deployer, router)
	ctx := context.Background()

	// Create previews for two projects.
	if _, err := mgr.Create(ctx, "proj-a", 1, "img:1", "branch-1"); err != nil {
		t.Fatalf("create A/1: %v", err)
	}
	if _, err := mgr.Create(ctx, "proj-a", 2, "img:2", "branch-2"); err != nil {
		t.Fatalf("create A/2: %v", err)
	}
	if _, err := mgr.Create(ctx, "proj-b", 1, "img:3", "branch-3"); err != nil {
		t.Fatalf("create B/1: %v", err)
	}

	listA := mgr.ListByProject("proj-a")
	if len(listA) != 2 {
		t.Errorf("ListByProject(proj-a) returned %d items, want 2", len(listA))
	}
	for _, env := range listA {
		if env.ProjectID != "proj-a" {
			t.Errorf("env.ProjectID = %q, want %q", env.ProjectID, "proj-a")
		}
	}

	listB := mgr.ListByProject("proj-b")
	if len(listB) != 1 {
		t.Errorf("ListByProject(proj-b) returned %d items, want 1", len(listB))
	}

	// Nonexistent project returns empty.
	listMissing := mgr.ListByProject("proj-missing")
	if len(listMissing) != 0 {
		t.Errorf("ListByProject(proj-missing) returned %d items, want 0", len(listMissing))
	}
}

func TestCleanup(t *testing.T) {
	deployer := newFakeDeployer()
	router := newFakeRouter()
	mgr := NewManager("preview.example.com", deployer, router)
	ctx := context.Background()

	// Set a very short TTL so the env expires immediately.
	mgr.SetTTL(1 * time.Millisecond)

	env, err := mgr.Create(ctx, "proj-1", 1, "img:v1", "branch")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Wait for the environment to expire.
	time.Sleep(10 * time.Millisecond)

	removed, errs := mgr.Cleanup(0)
	if len(errs) != 0 {
		t.Fatalf("cleanup errors: %v", errs)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if mgr.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after cleanup", mgr.Count())
	}

	// Environment should be gone.
	_, err = mgr.Get(env.ID)
	if err == nil {
		t.Error("expected error on Get after cleanup, got nil")
	}
}

func TestSetTTL(t *testing.T) {
	deployer := newFakeDeployer()
	router := newFakeRouter()
	mgr := NewManager("preview.example.com", deployer, router)

	// Negative TTL should default to 72h.
	mgr.SetTTL(-1)
	if mgr.ttl != 72*time.Hour {
		t.Errorf("ttl after SetTTL(-1) = %v, want %v", mgr.ttl, 72*time.Hour)
	}

	// Zero TTL should default to 72h.
	mgr.SetTTL(0)
	if mgr.ttl != 72*time.Hour {
		t.Errorf("ttl after SetTTL(0) = %v, want %v", mgr.ttl, 72*time.Hour)
	}

	// Positive TTL should be applied.
	mgr.SetTTL(24 * time.Hour)
	if mgr.ttl != 24*time.Hour {
		t.Errorf("ttl after SetTTL(24h) = %v, want %v", mgr.ttl, 24*time.Hour)
	}
}
