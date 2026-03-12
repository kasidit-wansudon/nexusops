package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector()
	require.NotNil(t, c)
	assert.NotNil(t, c.registry)
	assert.NotNil(t, c.httpRequestsTotal)
	assert.NotNil(t, c.httpRequestDuration)
	assert.NotNil(t, c.activeDeployments)
	assert.NotNil(t, c.buildDuration)
	assert.NotNil(t, c.deploymentStatus)
	assert.NotNil(t, c.containerCPU)
	assert.NotNil(t, c.containerMemory)
	assert.NotNil(t, c.metricSnapshot)
}

func TestRecordHTTPRequest(t *testing.T) {
	c := NewCollector()

	c.RecordHTTPRequest("GET", "/api/projects", "200", 0.125)
	c.RecordHTTPRequest("POST", "/api/deploy", "201", 0.350)
	c.RecordHTTPRequest("GET", "/api/projects", "200", 0.080)

	metrics := c.GetMetrics()

	// Two GET requests to /api/projects with status 200
	assert.Equal(t, float64(2), metrics["http_requests_total_GET_/api/projects_200"])
	// One POST request
	assert.Equal(t, float64(1), metrics["http_requests_total_POST_/api/deploy_201"])
	// Last duration for GET /api/projects
	assert.Equal(t, 0.080, metrics["http_request_duration_last_GET_/api/projects"])
	// Last duration for POST /api/deploy
	assert.Equal(t, 0.350, metrics["http_request_duration_last_POST_/api/deploy"])
}

func TestCollectorHandler(t *testing.T) {
	c := NewCollector()

	// Record some data so the handler has metrics to report
	c.RecordHTTPRequest("GET", "/health", "200", 0.001)
	c.SetActiveDeployments(3)

	handler := c.Handler()
	require.NotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	// Prometheus exposition format should contain our metric names
	assert.True(t, strings.Contains(body, "nexusops_http_requests_total"), "should contain http_requests_total metric")
	assert.True(t, strings.Contains(body, "nexusops_deployments_active_total"), "should contain active deployments metric")
}

func TestRecordBuildDuration(t *testing.T) {
	c := NewCollector()

	c.RecordBuild("my-app", "success", 45.2)
	c.RecordBuild("my-app", "failure", 12.5)
	c.RecordBuild("other-app", "success", 30.0)

	metrics := c.GetMetrics()

	assert.Equal(t, 12.5, metrics["build_duration_last_my-app"])
	assert.Equal(t, float64(1), metrics["build_success_total_my-app"])
	assert.Equal(t, float64(1), metrics["build_failure_total_my-app"])
	assert.Equal(t, 30.0, metrics["build_duration_last_other-app"])
	assert.Equal(t, float64(1), metrics["build_success_total_other-app"])
}

func TestRecordDeployment(t *testing.T) {
	c := NewCollector()

	c.SetActiveDeployments(5)
	metrics := c.GetMetrics()
	assert.Equal(t, float64(5), metrics["active_deployments"])

	c.SetDeploymentStatus("web-app", "production", "running")
	c.SetDeploymentStatus("api-svc", "staging", "stopped")
	c.SetDeploymentStatus("worker", "production", "failed")

	metrics = c.GetMetrics()
	assert.Equal(t, float64(1), metrics["deployment_status_web-app_production"])
	assert.Equal(t, float64(0), metrics["deployment_status_api-svc_staging"])
	assert.Equal(t, float64(-1), metrics["deployment_status_worker_production"])
}
