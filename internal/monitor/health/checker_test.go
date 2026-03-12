package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubCheck always returns a fixed result.
type stubCheck struct {
	status Status
	msg    string
}

func (s *stubCheck) Execute(_ context.Context) *Result {
	return &Result{
		Status:    s.status,
		Latency:   1 * time.Millisecond,
		Message:   s.msg,
		Timestamp: time.Now(),
		Details:   map[string]string{},
	}
}

func TestNewCheckerDefaultInterval(t *testing.T) {
	c := NewChecker(0)
	assert.NotNil(t, c)
	// Internal interval should default to 30s; we just verify it doesn't panic.
}

func TestRegisterAndGetStatus(t *testing.T) {
	c := NewChecker(100 * time.Millisecond)
	c.Register("db", &stubCheck{status: StatusHealthy, msg: "ok"})

	c.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	c.Stop()

	result := c.GetStatus("db")
	require.NotNil(t, result)
	assert.Equal(t, StatusHealthy, result.Status)
}

func TestGetAllStatuses(t *testing.T) {
	c := NewChecker(100 * time.Millisecond)
	c.Register("db", &stubCheck{status: StatusHealthy, msg: "ok"})
	c.Register("cache", &stubCheck{status: StatusUnhealthy, msg: "down"})

	c.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	c.Stop()

	all := c.GetAllStatuses()
	assert.Len(t, all, 2)
	assert.Equal(t, StatusHealthy, all["db"].Status)
	assert.Equal(t, StatusUnhealthy, all["cache"].Status)
}

func TestIsHealthyAllHealthy(t *testing.T) {
	c := NewChecker(100 * time.Millisecond)
	c.Register("a", &stubCheck{status: StatusHealthy, msg: "ok"})
	c.Register("b", &stubCheck{status: StatusHealthy, msg: "ok"})

	c.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	c.Stop()

	assert.True(t, c.IsHealthy())
}

func TestIsHealthyWithUnhealthy(t *testing.T) {
	c := NewChecker(100 * time.Millisecond)
	c.Register("a", &stubCheck{status: StatusHealthy, msg: "ok"})
	c.Register("b", &stubCheck{status: StatusUnhealthy, msg: "fail"})

	c.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	c.Stop()

	assert.False(t, c.IsHealthy())
}

func TestIsHealthyNoResults(t *testing.T) {
	c := NewChecker(1 * time.Hour)
	// No checks registered, no results => false
	assert.False(t, c.IsHealthy())
}

func TestOnResultCallback(t *testing.T) {
	c := NewChecker(100 * time.Millisecond)
	c.Register("svc", &stubCheck{status: StatusHealthy, msg: "ok"})

	called := make(chan string, 10)
	c.OnResult(func(name string, result *Result) {
		called <- name
	})

	c.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	c.Stop()

	select {
	case name := <-called:
		assert.Equal(t, "svc", name)
	default:
		t.Fatal("callback was never invoked")
	}
}

func TestHealthHandlerHealthy(t *testing.T) {
	c := NewChecker(100 * time.Millisecond)
	c.Register("db", &stubCheck{status: StatusHealthy, msg: "ok"})

	c.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	c.Stop()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp["status"])
}

func TestHealthHandlerUnhealthy(t *testing.T) {
	c := NewChecker(100 * time.Millisecond)
	c.Register("db", &stubCheck{status: StatusUnhealthy, msg: "timeout"})

	c.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	c.Stop()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestReadinessHandler(t *testing.T) {
	c := NewChecker(100 * time.Millisecond)
	c.Register("db", &stubCheck{status: StatusHealthy, msg: "ok"})

	c.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	c.Stop()

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	c.ReadinessHandler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestReadinessHandlerNotReady(t *testing.T) {
	c := NewChecker(1 * time.Hour)
	// No results yet => not ready
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	c.ReadinessHandler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestLivenessHandler(t *testing.T) {
	c := NewChecker(1 * time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	w := httptest.NewRecorder()
	c.LivenessHandler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp["status"])
}

func TestCommandCheckSuccess(t *testing.T) {
	check := &CommandCheck{
		Command: "echo",
		Args:    []string{"hello"},
		Timeout: 5 * time.Second,
	}

	result := check.Execute(context.Background())
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "command exited successfully", result.Message)
}

func TestCommandCheckFailure(t *testing.T) {
	check := &CommandCheck{
		Command: "false",
		Timeout: 5 * time.Second,
	}

	result := check.Execute(context.Background())
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "command failed")
}

func TestHTTPCheckAgainstTestServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	check := &HTTPCheck{
		URL:            srv.URL,
		ExpectedStatus: http.StatusOK,
		Timeout:        2 * time.Second,
	}

	result := check.Execute(context.Background())
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "OK", result.Message)
}

func TestHTTPCheckWrongStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	check := &HTTPCheck{
		URL:            srv.URL,
		ExpectedStatus: http.StatusOK,
		Timeout:        2 * time.Second,
	}

	result := check.Execute(context.Background())
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "unexpected status")
}

func TestStopWithoutStart(t *testing.T) {
	c := NewChecker(1 * time.Second)
	// Should not panic.
	c.Stop()
}
