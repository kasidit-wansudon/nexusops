package kubernetes

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ResourceRequirements defines CPU and memory constraints for a pod.
type ResourceRequirements struct {
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string
}

// HealthCheck defines a liveness / readiness probe specification.
type HealthCheck struct {
	Path                string
	Port                int
	InitialDelaySeconds int
	PeriodSeconds       int
	TimeoutSeconds      int
	FailureThreshold    int
}

// K8sDeployConfig carries every parameter needed to deploy a workload to
// Kubernetes.
type K8sDeployConfig struct {
	Name        string
	Namespace   string
	Image       string
	Replicas    int32
	Port        int32
	Env         map[string]string
	Resources   ResourceRequirements
	Labels      map[string]string
	HealthCheck *HealthCheck
}

// DeploymentStatus is the observed state of a Kubernetes Deployment.
type DeploymentStatus struct {
	Name              string
	Namespace         string
	Replicas          int32
	ReadyReplicas     int32
	UpdatedReplicas   int32
	AvailableReplicas int32
	Conditions        []DeploymentCondition
	Pods              []PodInfo
}

// DeploymentCondition mirrors appsv1.DeploymentCondition.
type DeploymentCondition struct {
	Type    string
	Status  string
	Reason  string
	Message string
}

// PodInfo is a lightweight view of a single pod.
type PodInfo struct {
	Name   string
	Phase  string
	Ready  bool
	IP     string
	Node   string
}

// k8sClient abstracts calls to the Kubernetes API so the deployer can be
// compiled without a live cluster.
type k8sClient struct {
	kubeconfig string
	mu         sync.RWMutex
	deploys    map[string]*storedDeploy
}

type storedDeploy struct {
	config    K8sDeployConfig
	status    DeploymentStatus
	createdAt time.Time
}

func newK8sClient(kubeconfig string) (*k8sClient, error) {
	if kubeconfig == "" {
		kubeconfig = "~/.kube/config"
	}
	return &k8sClient{
		kubeconfig: kubeconfig,
		deploys:    make(map[string]*storedDeploy),
	}, nil
}

func deployKey(namespace, name string) string {
	return namespace + "/" + name
}

// Deployer manages Kubernetes workloads.
type Deployer struct {
	k8sClient *k8sClient
}

// NewDeployer returns a Deployer configured with the given kubeconfig path.
func NewDeployer(kubeconfig string) (*Deployer, error) {
	c, err := newK8sClient(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("init k8s client: %w", err)
	}
	return &Deployer{k8sClient: c}, nil
}

// Deploy creates a Kubernetes Deployment, a Service, and optionally an
// Ingress resource for the supplied configuration.
func (d *Deployer) Deploy(ctx context.Context, config *K8sDeployConfig) error {
	if config == nil {
		return fmt.Errorf("config must not be nil")
	}
	if err := validateConfig(config); err != nil {
		return err
	}

	ns := config.Namespace
	if ns == "" {
		ns = "default"
	}
	config.Namespace = ns

	if config.Replicas <= 0 {
		config.Replicas = 1
	}

	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels["app.kubernetes.io/name"] = config.Name
	config.Labels["app.kubernetes.io/managed-by"] = "nexusops"

	// --- Create Deployment ---
	if err := d.createDeploymentResource(ctx, config); err != nil {
		return fmt.Errorf("create deployment: %w", err)
	}

	// --- Create Service ---
	if err := d.createServiceResource(ctx, config); err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	// --- Optionally create Ingress (when port is HTTP/HTTPS) ---
	if config.Port == 80 || config.Port == 443 || config.Port == 8080 {
		if err := d.createIngressResource(ctx, config); err != nil {
			return fmt.Errorf("create ingress: %w", err)
		}
	}

	return nil
}

// Update patches an existing Deployment with a new image and/or replica count.
func (d *Deployer) Update(ctx context.Context, config *K8sDeployConfig) error {
	if config == nil {
		return fmt.Errorf("config must not be nil")
	}
	if err := validateConfig(config); err != nil {
		return err
	}

	ns := config.Namespace
	if ns == "" {
		ns = "default"
	}

	key := deployKey(ns, config.Name)

	d.k8sClient.mu.Lock()
	defer d.k8sClient.mu.Unlock()

	stored, ok := d.k8sClient.deploys[key]
	if !ok {
		return fmt.Errorf("deployment %s not found in namespace %s", config.Name, ns)
	}

	if config.Image != "" {
		stored.config.Image = config.Image
		stored.status.UpdatedReplicas = 0 // rollout pending
	}
	if config.Replicas > 0 {
		stored.config.Replicas = config.Replicas
		stored.status.Replicas = config.Replicas
	}
	return nil
}

// Delete removes the Deployment, Service, and Ingress for the given name.
func (d *Deployer) Delete(ctx context.Context, namespace, name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if namespace == "" {
		namespace = "default"
	}

	key := deployKey(namespace, name)

	d.k8sClient.mu.Lock()
	defer d.k8sClient.mu.Unlock()

	if _, ok := d.k8sClient.deploys[key]; !ok {
		return fmt.Errorf("deployment %s not found in namespace %s", name, namespace)
	}
	delete(d.k8sClient.deploys, key)
	return nil
}

// GetStatus returns the observed state of a Deployment.
func (d *Deployer) GetStatus(ctx context.Context, namespace, name string) (*DeploymentStatus, error) {
	if namespace == "" {
		namespace = "default"
	}

	key := deployKey(namespace, name)

	d.k8sClient.mu.RLock()
	defer d.k8sClient.mu.RUnlock()

	stored, ok := d.k8sClient.deploys[key]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found in namespace %s", name, namespace)
	}

	status := stored.status // copy
	return &status, nil
}

// Scale adjusts the replica count for a Deployment.
func (d *Deployer) Scale(ctx context.Context, namespace, name string, replicas int32) error {
	if replicas < 0 {
		return fmt.Errorf("replicas must be non-negative, got %d", replicas)
	}
	if namespace == "" {
		namespace = "default"
	}

	key := deployKey(namespace, name)

	d.k8sClient.mu.Lock()
	defer d.k8sClient.mu.Unlock()

	stored, ok := d.k8sClient.deploys[key]
	if !ok {
		return fmt.Errorf("deployment %s not found in namespace %s", name, namespace)
	}

	stored.config.Replicas = replicas
	stored.status.Replicas = replicas
	return nil
}

// GetPodLogs returns a reader streaming logs from the given pod.
func (d *Deployer) GetPodLogs(ctx context.Context, namespace, podName string) (io.ReadCloser, error) {
	if podName == "" {
		return nil, fmt.Errorf("pod name is required")
	}
	if namespace == "" {
		namespace = "default"
	}

	// In production this would call coreV1.Pods(ns).GetLogs(podName, opts).Stream(ctx).
	// We return a placeholder reader so the interface compiles.
	logContent := fmt.Sprintf("[nexusops] streaming logs for pod %s/%s\n", namespace, podName)
	return io.NopCloser(strings.NewReader(logContent)), nil
}

// WaitForRollout polls the Deployment status until all replicas are available
// or the timeout elapses.
func (d *Deployer) WaitForRollout(ctx context.Context, namespace, name string, timeout time.Duration) error {
	if namespace == "" {
		namespace = "default"
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case t := <-ticker.C:
			if t.After(deadline) {
				return fmt.Errorf("timeout waiting for rollout of %s/%s after %s", namespace, name, timeout)
			}

			status, err := d.GetStatus(ctx, namespace, name)
			if err != nil {
				return err
			}

			if status.AvailableReplicas == status.Replicas && status.UpdatedReplicas == status.Replicas {
				return nil
			}

			// Check for failure conditions.
			for _, cond := range status.Conditions {
				if cond.Type == "Progressing" && cond.Status == "False" {
					return fmt.Errorf("rollout failed: %s", cond.Message)
				}
			}
		}
	}
}

// --- internal helpers ---

func validateConfig(c *K8sDeployConfig) error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.Image == "" {
		return fmt.Errorf("image is required")
	}
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535, got %d", c.Port)
	}
	return nil
}

func (d *Deployer) createDeploymentResource(ctx context.Context, config *K8sDeployConfig) error {
	key := deployKey(config.Namespace, config.Name)

	d.k8sClient.mu.Lock()
	defer d.k8sClient.mu.Unlock()

	if _, exists := d.k8sClient.deploys[key]; exists {
		return fmt.Errorf("deployment %s already exists in namespace %s", config.Name, config.Namespace)
	}

	sd := &storedDeploy{
		config:    *config,
		createdAt: time.Now().UTC(),
		status: DeploymentStatus{
			Name:              config.Name,
			Namespace:         config.Namespace,
			Replicas:          config.Replicas,
			ReadyReplicas:     config.Replicas,
			UpdatedReplicas:   config.Replicas,
			AvailableReplicas: config.Replicas,
			Conditions: []DeploymentCondition{
				{Type: "Available", Status: "True", Reason: "MinimumReplicasAvailable"},
				{Type: "Progressing", Status: "True", Reason: "NewReplicaSetAvailable"},
			},
		},
	}

	// Generate synthetic pods.
	pods := make([]PodInfo, 0, config.Replicas)
	for i := int32(0); i < config.Replicas; i++ {
		pods = append(pods, PodInfo{
			Name:  fmt.Sprintf("%s-%d", config.Name, i),
			Phase: "Running",
			Ready: true,
			IP:    fmt.Sprintf("10.0.%d.%d", i/256, i%256+2),
			Node:  "node-0",
		})
	}
	sd.status.Pods = pods

	d.k8sClient.deploys[key] = sd
	return nil
}

func (d *Deployer) createServiceResource(ctx context.Context, config *K8sDeployConfig) error {
	// In a real implementation this would build a corev1.Service and call
	// coreV1.Services(ns).Create(ctx, svc, metav1.CreateOptions{}).
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

func (d *Deployer) createIngressResource(ctx context.Context, config *K8sDeployConfig) error {
	// In a real implementation this would build a networkingv1.Ingress and
	// call networkingV1.Ingresses(ns).Create(ctx, ing, metav1.CreateOptions{}).
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}
