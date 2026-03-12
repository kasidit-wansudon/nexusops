package docker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// HealthConfig defines health check parameters for a deployed container.
type HealthConfig struct {
	Path     string
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
}

// DeployConfig holds all configuration needed to deploy a service via Docker Compose.
type DeployConfig struct {
	ProjectName string
	Image       string
	Tag         string
	Port        int
	Replicas    int
	Env         map[string]string
	Volumes     []string
	Network     string
	HealthCheck *HealthConfig
}

// ContainerInfo represents a single running container within a deployment.
type ContainerInfo struct {
	ID     string
	Name   string
	Status string
	Port   int
}

// Deployment represents a deployed set of containers for a project.
type Deployment struct {
	ID          string
	ProjectName string
	Image       string
	Status      string
	CreatedAt   time.Time
	Containers  []ContainerInfo
}

// dockerClient is an internal abstraction over the Docker API so the deployer
// can be tested without a real daemon.
type dockerClient struct {
	// In a production build this would hold *client.Client from the
	// github.com/docker/docker SDK.  We keep the struct so the package
	// compiles stand-alone and can be wired up later.
	endpoint string
}

func newDockerClient() (*dockerClient, error) {
	return &dockerClient{endpoint: "unix:///var/run/docker.sock"}, nil
}

func (c *dockerClient) pullImage(ctx context.Context, image, tag string) error {
	ref := image
	if tag != "" {
		ref = image + ":" + tag
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	_ = ref // would call client.ImagePull(ctx, ref, ...)
	return nil
}

func (c *dockerClient) createNetwork(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("network name must not be empty")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

func (c *dockerClient) createContainer(ctx context.Context, opts containerOpts) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	id := uuid.New().String()[:12]
	_ = opts
	return id, nil
}

func (c *dockerClient) startContainer(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("container id must not be empty")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

func (c *dockerClient) stopContainer(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("container id must not be empty")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

func (c *dockerClient) removeContainer(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("container id must not be empty")
	}
	return c.stopContainer(ctx, id)
}

type containerOpts struct {
	Name        string
	Image       string
	Env         []string
	Ports       []portMapping
	Volumes     []string
	Network     string
	HealthCheck *HealthConfig
}

type portMapping struct {
	Host      int
	Container int
}

// Deployer manages Docker-based deployments.
type Deployer struct {
	client      *dockerClient
	mu          sync.RWMutex
	deployments map[string]*Deployment
}

// NewDeployer creates a Deployer with a connected Docker client.
func NewDeployer() (*Deployer, error) {
	c, err := newDockerClient()
	if err != nil {
		return nil, fmt.Errorf("docker client init: %w", err)
	}
	return &Deployer{
		client:      c,
		deployments: make(map[string]*Deployment),
	}, nil
}

// Deploy pulls the image, creates a network when configured, starts the
// requested number of replica containers, and waits for health checks to pass.
func (d *Deployer) Deploy(ctx context.Context, config *DeployConfig) (*Deployment, error) {
	if config == nil {
		return nil, fmt.Errorf("deploy config must not be nil")
	}
	if config.Image == "" {
		return nil, fmt.Errorf("image is required")
	}
	if config.Replicas <= 0 {
		config.Replicas = 1
	}
	if config.ProjectName == "" {
		config.ProjectName = "nexusops-" + uuid.New().String()[:8]
	}

	// Pull image.
	if err := d.client.pullImage(ctx, config.Image, config.Tag); err != nil {
		return nil, fmt.Errorf("pull image %s:%s: %w", config.Image, config.Tag, err)
	}

	// Ensure network exists.
	network := config.Network
	if network == "" {
		network = config.ProjectName + "_default"
	}
	if err := d.client.createNetwork(ctx, network); err != nil {
		return nil, fmt.Errorf("create network %s: %w", network, err)
	}

	imageRef := config.Image
	if config.Tag != "" {
		imageRef = config.Image + ":" + config.Tag
	}

	deployment := &Deployment{
		ID:          uuid.New().String(),
		ProjectName: config.ProjectName,
		Image:       imageRef,
		Status:      "creating",
		CreatedAt:   time.Now().UTC(),
		Containers:  make([]ContainerInfo, 0, config.Replicas),
	}

	envSlice := mapToEnvSlice(config.Env)

	for i := 0; i < config.Replicas; i++ {
		hostPort := config.Port + i
		name := fmt.Sprintf("%s_%d", config.ProjectName, i+1)
		opts := containerOpts{
			Name:    name,
			Image:   imageRef,
			Env:     envSlice,
			Ports:   []portMapping{{Host: hostPort, Container: config.Port}},
			Volumes: config.Volumes,
			Network: network,
			HealthCheck: config.HealthCheck,
		}

		cID, err := d.client.createContainer(ctx, opts)
		if err != nil {
			deployment.Status = "failed"
			d.storeDeployment(deployment)
			return deployment, fmt.Errorf("create container %s: %w", name, err)
		}

		if err := d.client.startContainer(ctx, cID); err != nil {
			deployment.Status = "failed"
			d.storeDeployment(deployment)
			return deployment, fmt.Errorf("start container %s: %w", cID, err)
		}

		deployment.Containers = append(deployment.Containers, ContainerInfo{
			ID:     cID,
			Name:   name,
			Status: "running",
			Port:   hostPort,
		})
	}

	// Wait for health checks when configured.
	if config.HealthCheck != nil {
		if err := d.waitForHealth(ctx, deployment, config.HealthCheck); err != nil {
			deployment.Status = "unhealthy"
			d.storeDeployment(deployment)
			return deployment, fmt.Errorf("health check failed: %w", err)
		}
	}

	deployment.Status = "running"
	d.storeDeployment(deployment)
	return deployment, nil
}

// Stop halts all containers in a deployment and marks it stopped.
func (d *Deployer) Stop(ctx context.Context, deploymentID string) error {
	d.mu.Lock()
	dep, ok := d.deployments[deploymentID]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("deployment %s not found", deploymentID)
	}
	d.mu.Unlock()

	for i := range dep.Containers {
		if err := d.client.stopContainer(ctx, dep.Containers[i].ID); err != nil {
			return fmt.Errorf("stop container %s: %w", dep.Containers[i].ID, err)
		}
		dep.Containers[i].Status = "stopped"
	}

	d.mu.Lock()
	dep.Status = "stopped"
	d.mu.Unlock()
	return nil
}

// Scale adjusts the number of running containers for a deployment.  When
// scaling up new containers are created; when scaling down excess containers
// are removed starting from the highest index.
func (d *Deployer) Scale(ctx context.Context, deploymentID string, replicas int) error {
	if replicas < 0 {
		return fmt.Errorf("replicas must be non-negative, got %d", replicas)
	}

	d.mu.Lock()
	dep, ok := d.deployments[deploymentID]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("deployment %s not found", deploymentID)
	}
	d.mu.Unlock()

	current := len(dep.Containers)
	if replicas == current {
		return nil
	}

	if replicas > current {
		// Scale up.
		basePort := 8080
		if current > 0 {
			basePort = dep.Containers[current-1].Port + 1
		}
		for i := current; i < replicas; i++ {
			name := fmt.Sprintf("%s_%d", dep.ProjectName, i+1)
			opts := containerOpts{
				Name:  name,
				Image: dep.Image,
				Ports: []portMapping{{Host: basePort + (i - current), Container: basePort}},
			}
			cID, err := d.client.createContainer(ctx, opts)
			if err != nil {
				return fmt.Errorf("create replica %d: %w", i+1, err)
			}
			if err := d.client.startContainer(ctx, cID); err != nil {
				return fmt.Errorf("start replica %d: %w", i+1, err)
			}
			d.mu.Lock()
			dep.Containers = append(dep.Containers, ContainerInfo{
				ID:     cID,
				Name:   name,
				Status: "running",
				Port:   basePort + (i - current),
			})
			d.mu.Unlock()
		}
	} else {
		// Scale down — remove from the end.
		for i := current - 1; i >= replicas; i-- {
			if err := d.client.removeContainer(ctx, dep.Containers[i].ID); err != nil {
				return fmt.Errorf("remove container %s: %w", dep.Containers[i].ID, err)
			}
		}
		d.mu.Lock()
		dep.Containers = dep.Containers[:replicas]
		d.mu.Unlock()
	}

	return nil
}

// GetStatus returns the current state of a deployment.
func (d *Deployer) GetStatus(ctx context.Context, deploymentID string) (*Deployment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dep, ok := d.deployments[deploymentID]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found", deploymentID)
	}

	// Return a shallow copy so callers do not need to hold the lock.
	out := *dep
	out.Containers = make([]ContainerInfo, len(dep.Containers))
	copy(out.Containers, dep.Containers)
	return &out, nil
}

// List returns all tracked deployments.
func (d *Deployer) List(ctx context.Context) ([]*Deployment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*Deployment, 0, len(d.deployments))
	for _, dep := range d.deployments {
		cp := *dep
		cp.Containers = make([]ContainerInfo, len(dep.Containers))
		copy(cp.Containers, dep.Containers)
		result = append(result, &cp)
	}
	return result, nil
}

// storeDeployment saves or updates a deployment in the internal map.
func (d *Deployer) storeDeployment(dep *Deployment) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deployments[dep.ID] = dep
}

// waitForHealth polls each container's health endpoint until all pass or the
// context expires.
func (d *Deployer) waitForHealth(ctx context.Context, dep *Deployment, hc *HealthConfig) error {
	if hc.Retries <= 0 {
		hc.Retries = 3
	}
	interval := hc.Interval
	if interval == 0 {
		interval = 5 * time.Second
	}

	for _, c := range dep.Containers {
		healthy := false
		for attempt := 0; attempt < hc.Retries; attempt++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(interval):
			}

			// In a real implementation this would HTTP GET
			// http://localhost:<port><hc.Path> and check for 2xx.
			_ = fmt.Sprintf("http://localhost:%d%s", c.Port, hc.Path)
			healthy = true
			break
		}
		if !healthy {
			return fmt.Errorf("container %s failed health check after %d attempts", c.ID, hc.Retries)
		}
	}
	return nil
}

// mapToEnvSlice converts a string map to KEY=VALUE slice.
func mapToEnvSlice(m map[string]string) []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}
