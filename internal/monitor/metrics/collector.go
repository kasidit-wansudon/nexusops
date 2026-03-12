// Package metrics provides Prometheus metric collection for the NexusOps platform.
package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Collector manages Prometheus metrics for the NexusOps platform, including
// HTTP request tracking, build duration, deployment status, and container resource usage.
type Collector struct {
	registry *prometheus.Registry

	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	activeDeployments   prometheus.Gauge
	buildDuration       *prometheus.HistogramVec
	deploymentStatus    *prometheus.GaugeVec
	containerCPU        *prometheus.GaugeVec
	containerMemory     *prometheus.GaugeVec

	mu             sync.RWMutex
	metricSnapshot map[string]float64
}

// NewCollector creates a new Collector with all Prometheus metrics registered
// to a dedicated registry.
func NewCollector() *Collector {
	reg := prometheus.NewRegistry()

	httpRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "nexusops",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests processed, partitioned by method, path, and status.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "nexusops",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Histogram of HTTP request latencies in seconds.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"method", "path", "status"},
	)

	activeDeployments := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "nexusops",
			Subsystem: "deployments",
			Name:      "active_total",
			Help:      "Current number of active deployments across all environments.",
		},
	)

	buildDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "nexusops",
			Subsystem: "build",
			Name:      "duration_seconds",
			Help:      "Histogram of build durations in seconds, partitioned by project and status.",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600, 1200, 1800},
		},
		[]string{"project", "status"},
	)

	deploymentStatus := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "nexusops",
			Subsystem: "deployment",
			Name:      "status",
			Help:      "Current deployment status gauge. 1 = running, 0 = stopped, -1 = failed.",
		},
		[]string{"project", "environment", "status"},
	)

	containerCPU := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "nexusops",
			Subsystem: "container",
			Name:      "cpu_usage_ratio",
			Help:      "Current CPU usage ratio (0.0 to 1.0+) per container.",
		},
		[]string{"container_id"},
	)

	containerMemory := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "nexusops",
			Subsystem: "container",
			Name:      "memory_usage_bytes",
			Help:      "Current memory usage in bytes per container.",
		},
		[]string{"container_id"},
	)

	reg.MustRegister(httpRequestsTotal)
	reg.MustRegister(httpRequestDuration)
	reg.MustRegister(activeDeployments)
	reg.MustRegister(buildDuration)
	reg.MustRegister(deploymentStatus)
	reg.MustRegister(containerCPU)
	reg.MustRegister(containerMemory)

	return &Collector{
		registry:            reg,
		httpRequestsTotal:   httpRequestsTotal,
		httpRequestDuration: httpRequestDuration,
		activeDeployments:   activeDeployments,
		buildDuration:       buildDuration,
		deploymentStatus:    deploymentStatus,
		containerCPU:        containerCPU,
		containerMemory:     containerMemory,
		metricSnapshot:      make(map[string]float64),
	}
}

// RecordHTTPRequest records a completed HTTP request with the given method,
// path, status code, and duration in seconds.
func (c *Collector) RecordHTTPRequest(method, path, status string, duration float64) {
	c.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
	c.httpRequestDuration.WithLabelValues(method, path, status).Observe(duration)

	c.mu.Lock()
	defer c.mu.Unlock()
	key := "http_requests_total_" + method + "_" + path + "_" + status
	c.metricSnapshot[key]++
	c.metricSnapshot["http_request_duration_last_"+method+"_"+path] = duration
}

// RecordBuild records a completed build with the project name, final status,
// and duration in seconds.
func (c *Collector) RecordBuild(project, status string, duration float64) {
	c.buildDuration.WithLabelValues(project, status).Observe(duration)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.metricSnapshot["build_duration_last_"+project] = duration
	if status == "success" {
		c.metricSnapshot["build_success_total_"+project]++
	} else {
		c.metricSnapshot["build_failure_total_"+project]++
	}
}

// SetActiveDeployments sets the current count of active deployments.
func (c *Collector) SetActiveDeployments(count float64) {
	c.activeDeployments.Set(count)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.metricSnapshot["active_deployments"] = count
}

// SetDeploymentStatus sets the status gauge for a specific project/environment
// combination. The status string is stored as a label; the gauge value encodes
// the state: "running" = 1, "stopped" = 0, "failed" = -1.
func (c *Collector) SetDeploymentStatus(project, env, status string) {
	var value float64
	switch status {
	case "running":
		value = 1
	case "stopped":
		value = 0
	case "failed":
		value = -1
	default:
		value = 0
	}
	c.deploymentStatus.WithLabelValues(project, env, status).Set(value)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.metricSnapshot["deployment_status_"+project+"_"+env] = value
}

// SetContainerMetrics sets the current CPU and memory usage for a container.
// CPU is expressed as a ratio (e.g., 0.5 = 50%) and memory in bytes.
func (c *Collector) SetContainerMetrics(containerID string, cpu, memory float64) {
	c.containerCPU.WithLabelValues(containerID).Set(cpu)
	c.containerMemory.WithLabelValues(containerID).Set(memory)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.metricSnapshot["container_cpu_"+containerID] = cpu
	c.metricSnapshot["container_memory_"+containerID] = memory
}

// Handler returns an http.Handler that exposes all registered Prometheus
// metrics in the standard exposition format.
func (c *Collector) Handler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{
		EnableOpenMetrics:   true,
		MaxRequestsInFlight: 10,
	})
}

// GetMetrics returns a snapshot of the most recently recorded metric values
// as a flat key-value map. This is primarily useful for internal rule evaluation
// and testing.
func (c *Collector) GetMetrics() map[string]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := make(map[string]float64, len(c.metricSnapshot))
	for k, v := range c.metricSnapshot {
		snapshot[k] = v
	}
	return snapshot
}

// Registry returns the underlying Prometheus registry, allowing external
// components to register additional custom metrics.
func (c *Collector) Registry() *prometheus.Registry {
	return c.registry
}

// ResetMetrics clears the internal metric snapshot. This does not reset the
// Prometheus counters/histograms themselves.
func (c *Collector) ResetMetrics() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metricSnapshot = make(map[string]float64)
}
