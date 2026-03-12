package rollback

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager(15 * time.Second)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.healthCheckInterval != 15*time.Second {
		t.Errorf("healthCheckInterval = %v, want %v", mgr.healthCheckInterval, 15*time.Second)
	}
	if mgr.failureThreshold != 3 {
		t.Errorf("failureThreshold = %d, want 3", mgr.failureThreshold)
	}

	// Zero interval should default to 10s.
	mgr2 := NewManager(0)
	if mgr2.healthCheckInterval != 10*time.Second {
		t.Errorf("healthCheckInterval for 0 = %v, want %v", mgr2.healthCheckInterval, 10*time.Second)
	}

	// Negative interval should default to 10s.
	mgr3 := NewManager(-5 * time.Second)
	if mgr3.healthCheckInterval != 10*time.Second {
		t.Errorf("healthCheckInterval for -5s = %v, want %v", mgr3.healthCheckInterval, 10*time.Second)
	}
}

func TestRecordDeployment(t *testing.T) {
	mgr := NewManager(10 * time.Second)

	record := &DeploymentRecord{
		ID:        "dep-1",
		ProjectID: "proj-1",
		Version:   "v1.0.0",
		Image:     "myapp:v1.0.0",
		Config:    map[string]string{"replicas": "3"},
		Status:    "success",
	}
	mgr.RecordDeployment(record)

	// Timestamp should be set automatically when zero.
	if record.Timestamp.IsZero() {
		t.Error("expected Timestamp to be set automatically, got zero")
	}

	history := mgr.GetHistory("proj-1")
	if len(history) != 1 {
		t.Fatalf("GetHistory returned %d records, want 1", len(history))
	}
	if history[0].ID != "dep-1" {
		t.Errorf("ID = %q, want %q", history[0].ID, "dep-1")
	}
	if history[0].Version != "v1.0.0" {
		t.Errorf("Version = %q, want %q", history[0].Version, "v1.0.0")
	}
	if history[0].Status != "success" {
		t.Errorf("Status = %q, want %q", history[0].Status, "success")
	}

	// Nil record should be a no-op.
	mgr.RecordDeployment(nil)
	if len(mgr.GetHistory("proj-1")) != 1 {
		t.Error("recording nil should not add to history")
	}

	// Record with an explicit timestamp should keep it.
	explicit := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	r2 := &DeploymentRecord{
		ID:        "dep-2",
		ProjectID: "proj-1",
		Version:   "v2.0.0",
		Image:     "myapp:v2.0.0",
		Status:    "success",
		Timestamp: explicit,
	}
	mgr.RecordDeployment(r2)
	if r2.Timestamp != explicit {
		t.Errorf("explicit Timestamp was overwritten: got %v, want %v", r2.Timestamp, explicit)
	}
}

func TestGetHistory(t *testing.T) {
	mgr := NewManager(10 * time.Second)

	// No history initially.
	if got := mgr.GetHistory("proj-1"); got != nil {
		t.Errorf("expected nil for empty history, got %v", got)
	}

	// Record multiple deployments.
	versions := []string{"v1.0.0", "v1.1.0", "v1.2.0"}
	for i, v := range versions {
		mgr.RecordDeployment(&DeploymentRecord{
			ID:        "dep-" + v,
			ProjectID: "proj-1",
			Version:   v,
			Image:     "myapp:" + v,
			Status:    "success",
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	history := mgr.GetHistory("proj-1")
	if len(history) != 3 {
		t.Fatalf("GetHistory returned %d records, want 3", len(history))
	}

	// History should be ordered oldest to newest.
	for i, v := range versions {
		if history[i].Version != v {
			t.Errorf("history[%d].Version = %q, want %q", i, history[i].Version, v)
		}
	}

	// History for a different project should be empty.
	if got := mgr.GetHistory("proj-other"); got != nil {
		t.Errorf("expected nil for proj-other, got %d records", len(got))
	}
}

func TestRollback(t *testing.T) {
	mgr := NewManager(10 * time.Second)

	var redeployedRecord *DeploymentRecord
	mgr.SetRedeployFunc(func(ctx context.Context, record *DeploymentRecord) error {
		redeployedRecord = record
		return nil
	})

	// Record two successful deployments.
	mgr.RecordDeployment(&DeploymentRecord{
		ID:        "dep-1",
		ProjectID: "proj-1",
		Version:   "v1.0.0",
		Image:     "myapp:v1.0.0",
		Status:    "success",
		Timestamp: time.Now().Add(-2 * time.Minute),
	})
	mgr.RecordDeployment(&DeploymentRecord{
		ID:        "dep-2",
		ProjectID: "proj-1",
		Version:   "v2.0.0",
		Image:     "myapp:v2.0.0",
		Status:    "success",
		Timestamp: time.Now().Add(-1 * time.Minute),
	})

	ctx := context.Background()
	result, err := mgr.Rollback(ctx, "proj-1")
	if err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Rollback returned nil result")
	}

	// The redeployed version should be v1.0.0.
	if redeployedRecord == nil {
		t.Fatal("redeploy function was not called")
	}
	if redeployedRecord.Version != "v1.0.0" {
		t.Errorf("redeployed version = %q, want %q", redeployedRecord.Version, "v1.0.0")
	}

	// The rollback record should reference v1.0.0.
	if result.Version != "v1.0.0" {
		t.Errorf("result.Version = %q, want %q", result.Version, "v1.0.0")
	}
	if result.Status != "success" {
		t.Errorf("result.Status = %q, want %q", result.Status, "success")
	}
	if !strings.Contains(result.ID, "rollback") {
		t.Errorf("result.ID = %q, want it to contain %q", result.ID, "rollback")
	}

	// History should now have 3 entries.
	history := mgr.GetHistory("proj-1")
	if len(history) != 3 {
		t.Errorf("history has %d records, want 3", len(history))
	}
}

func TestRollback_NoHistory(t *testing.T) {
	mgr := NewManager(10 * time.Second)
	mgr.SetRedeployFunc(func(ctx context.Context, record *DeploymentRecord) error {
		return nil
	})

	ctx := context.Background()

	// No history at all should fail.
	_, err := mgr.Rollback(ctx, "proj-empty")
	if err == nil {
		t.Fatal("expected error for empty history, got nil")
	}

	// Only one deployment -- no previous version to roll back to.
	mgr.RecordDeployment(&DeploymentRecord{
		ID:        "dep-1",
		ProjectID: "proj-single",
		Version:   "v1.0.0",
		Image:     "myapp:v1.0.0",
		Status:    "success",
		Timestamp: time.Now(),
	})
	_, err = mgr.Rollback(ctx, "proj-single")
	if err == nil {
		t.Fatal("expected error for single deployment, got nil")
	}
}

func TestRollback_NoRedeployFunc(t *testing.T) {
	mgr := NewManager(10 * time.Second)

	// Record two deployments but do NOT set the redeploy function.
	mgr.RecordDeployment(&DeploymentRecord{
		ID:        "dep-1",
		ProjectID: "proj-1",
		Version:   "v1.0.0",
		Image:     "myapp:v1.0.0",
		Status:    "success",
		Timestamp: time.Now().Add(-2 * time.Minute),
	})
	mgr.RecordDeployment(&DeploymentRecord{
		ID:        "dep-2",
		ProjectID: "proj-1",
		Version:   "v2.0.0",
		Image:     "myapp:v2.0.0",
		Status:    "success",
		Timestamp: time.Now(),
	})

	_, err := mgr.Rollback(context.Background(), "proj-1")
	if err == nil {
		t.Fatal("expected error when redeploy function not set, got nil")
	}
	if !strings.Contains(err.Error(), "redeploy function not set") {
		t.Errorf("error = %q, want substring %q", err.Error(), "redeploy function not set")
	}
}

func TestRollback_RedeployError(t *testing.T) {
	mgr := NewManager(10 * time.Second)
	mgr.SetRedeployFunc(func(ctx context.Context, record *DeploymentRecord) error {
		return fmt.Errorf("deploy failed")
	})

	mgr.RecordDeployment(&DeploymentRecord{
		ID:        "dep-1",
		ProjectID: "proj-1",
		Version:   "v1.0.0",
		Image:     "myapp:v1.0.0",
		Status:    "success",
		Timestamp: time.Now().Add(-2 * time.Minute),
	})
	mgr.RecordDeployment(&DeploymentRecord{
		ID:        "dep-2",
		ProjectID: "proj-1",
		Version:   "v2.0.0",
		Image:     "myapp:v2.0.0",
		Status:    "success",
		Timestamp: time.Now(),
	})

	_, err := mgr.Rollback(context.Background(), "proj-1")
	if err == nil {
		t.Fatal("expected error from redeploy failure, got nil")
	}
	if !strings.Contains(err.Error(), "rollback redeploy") {
		t.Errorf("error = %q, want substring %q", err.Error(), "rollback redeploy")
	}
}

func TestGetLatest(t *testing.T) {
	mgr := NewManager(10 * time.Second)

	// No history.
	_, err := mgr.GetLatest("proj-1")
	if err == nil {
		t.Fatal("expected error for empty history, got nil")
	}

	mgr.RecordDeployment(&DeploymentRecord{
		ID:        "dep-1",
		ProjectID: "proj-1",
		Version:   "v1.0.0",
		Image:     "myapp:v1.0.0",
		Status:    "success",
		Timestamp: time.Now().Add(-1 * time.Minute),
	})
	mgr.RecordDeployment(&DeploymentRecord{
		ID:        "dep-2",
		ProjectID: "proj-1",
		Version:   "v2.0.0",
		Image:     "myapp:v2.0.0",
		Status:    "success",
		Timestamp: time.Now(),
	})

	latest, err := mgr.GetLatest("proj-1")
	if err != nil {
		t.Fatalf("GetLatest: %v", err)
	}
	if latest.Version != "v2.0.0" {
		t.Errorf("latest.Version = %q, want %q", latest.Version, "v2.0.0")
	}
	if latest.ID != "dep-2" {
		t.Errorf("latest.ID = %q, want %q", latest.ID, "dep-2")
	}
}
