package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
)

// DockerClient wraps the Docker SDK client with higher-level operations
// needed by the NexusOps platform.
type DockerClient struct {
	cli *client.Client
}

// ContainerConfig holds the parameters for creating a new container.
type ContainerConfig struct {
	Image   string            `json:"image"`
	Name    string            `json:"name"`
	Env     map[string]string `json:"env"`
	Ports   map[string]string `json:"ports"` // container_port -> host_port
	Volumes map[string]string `json:"volumes"`
	Cmd     []string          `json:"cmd"`
	WorkDir string            `json:"work_dir"`
	Labels  map[string]string `json:"labels"`
}

// ContainerInfo summarizes a running or stopped container.
type ContainerInfo struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	State   string            `json:"state"`
	Status  string            `json:"status"`
	Ports   []PortBinding     `json:"ports"`
	Labels  map[string]string `json:"labels"`
	Created time.Time         `json:"created"`
}

// PortBinding describes a port mapping.
type PortBinding struct {
	ContainerPort string `json:"container_port"`
	HostPort      string `json:"host_port"`
	Protocol      string `json:"protocol"`
}

// BuildConfig holds parameters for building a Docker image.
type BuildConfig struct {
	ContextPath string            `json:"context_path"`
	Dockerfile  string            `json:"dockerfile"`
	Tags        []string          `json:"tags"`
	BuildArgs   map[string]string `json:"build_args"`
	NoCache     bool              `json:"no_cache"`
	Target      string            `json:"target"`
}

// NewDockerClient creates a DockerClient using environment defaults
// (DOCKER_HOST, DOCKER_API_VERSION, etc.).
func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker: creating client: %w", err)
	}
	return &DockerClient{cli: cli}, nil
}

// NewDockerClientWithHost creates a DockerClient that connects to a specific
// daemon endpoint.
func NewDockerClientWithHost(host string) (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker: creating client for host %s: %w", host, err)
	}
	return &DockerClient{cli: cli}, nil
}

// APIClient returns the underlying Docker API client, which implements
// client.APIClient. Use this when a lower-level interface is required.
func (d *DockerClient) APIClient() client.APIClient {
	return d.cli
}

// Close releases the underlying Docker client resources.
func (d *DockerClient) Close() error {
	return d.cli.Close()
}

// PullImage pulls a container image from a registry.
func (d *DockerClient) PullImage(ctx context.Context, imageName string) error {
	reader, err := d.cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("docker: pulling image %s: %w", imageName, err)
	}
	defer reader.Close()

	// Consume the pull output to completion.
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("docker: reading pull output for %s: %w", imageName, err)
	}
	return nil
}

// CreateContainer creates a new container from the given configuration and
// returns its ID.
func (d *DockerClient) CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error) {
	// Build environment slice.
	var envSlice []string
	for k, v := range cfg.Env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}

	// Build port bindings.
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for containerPort, hostPort := range cfg.Ports {
		cp, err := nat.NewPort("tcp", containerPort)
		if err != nil {
			return "", fmt.Errorf("docker: parsing container port %s: %w", containerPort, err)
		}
		exposedPorts[cp] = struct{}{}
		portBindings[cp] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: hostPort},
		}
	}

	// Build volume binds.
	var binds []string
	for hostPath, containerPath := range cfg.Volumes {
		binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	containerCfg := &container.Config{
		Image:        cfg.Image,
		Env:          envSlice,
		Cmd:          cfg.Cmd,
		WorkingDir:   cfg.WorkDir,
		ExposedPorts: exposedPorts,
		Labels:       cfg.Labels,
	}

	hostCfg := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        binds,
	}

	resp, err := d.cli.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, cfg.Name)
	if err != nil {
		return "", fmt.Errorf("docker: creating container %s: %w", cfg.Name, err)
	}

	return resp.ID, nil
}

// StartContainer starts a stopped container by ID.
func (d *DockerClient) StartContainer(ctx context.Context, id string) error {
	if err := d.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return fmt.Errorf("docker: starting container %s: %w", id, err)
	}
	return nil
}

// StopContainer gracefully stops a running container with the given timeout.
func (d *DockerClient) StopContainer(ctx context.Context, id string, timeout time.Duration) error {
	timeoutSec := int(timeout.Seconds())
	options := container.StopOptions{Timeout: &timeoutSec}
	if err := d.cli.ContainerStop(ctx, id, options); err != nil {
		return fmt.Errorf("docker: stopping container %s: %w", id, err)
	}
	return nil
}

// RemoveContainer removes a container. If force is true, a running container
// will be killed first.
func (d *DockerClient) RemoveContainer(ctx context.Context, id string, force bool) error {
	opts := container.RemoveOptions{
		Force:         force,
		RemoveVolumes: true,
	}
	if err := d.cli.ContainerRemove(ctx, id, opts); err != nil {
		return fmt.Errorf("docker: removing container %s: %w", id, err)
	}
	return nil
}

// ContainerLogs returns an io.ReadCloser streaming the combined stdout/stderr
// logs of a container.
func (d *DockerClient) ContainerLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
	}
	reader, err := d.cli.ContainerLogs(ctx, id, opts)
	if err != nil {
		return nil, fmt.Errorf("docker: getting logs for container %s: %w", id, err)
	}
	return reader, nil
}

// ListContainers returns information about all containers (running and stopped).
func (d *DockerClient) ListContainers(ctx context.Context) ([]ContainerInfo, error) {
	containers, err := d.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("docker: listing containers: %w", err)
	}

	infos := make([]ContainerInfo, 0, len(containers))
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}

		var ports []PortBinding
		for _, p := range c.Ports {
			ports = append(ports, PortBinding{
				ContainerPort: fmt.Sprintf("%d", p.PrivatePort),
				HostPort:      fmt.Sprintf("%d", p.PublicPort),
				Protocol:      p.Type,
			})
		}

		infos = append(infos, ContainerInfo{
			ID:      c.ID[:12],
			Name:    name,
			Image:   c.Image,
			State:   c.State,
			Status:  c.Status,
			Ports:   ports,
			Labels:  c.Labels,
			Created: time.Unix(c.Created, 0),
		})
	}
	return infos, nil
}

// WaitForContainer blocks until the container exits and returns its exit code.
func (d *DockerClient) WaitForContainer(ctx context.Context, id string) (int64, error) {
	statusCh, errCh := d.cli.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return -1, fmt.Errorf("docker: waiting for container %s: %w", id, err)
		}
	case status := <-statusCh:
		if status.Error != nil {
			return status.StatusCode, fmt.Errorf("docker: container %s exited with error: %s", id, status.Error.Message)
		}
		return status.StatusCode, nil
	case <-ctx.Done():
		return -1, ctx.Err()
	}
	return -1, nil
}

// BuildImage builds a Docker image from the specified build context.
func (d *DockerClient) BuildImage(ctx context.Context, cfg BuildConfig) error {
	dockerfile := cfg.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	tar, err := archive.TarWithOptions(cfg.ContextPath, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("docker: creating build context tar from %s: %w", cfg.ContextPath, err)
	}
	defer tar.Close()

	buildArgs := make(map[string]*string)
	for k, v := range cfg.BuildArgs {
		val := v // avoid capturing loop variable
		buildArgs[k] = &val
	}

	opts := types.ImageBuildOptions{
		Tags:       cfg.Tags,
		Dockerfile: dockerfile,
		BuildArgs:  buildArgs,
		NoCache:    cfg.NoCache,
		Remove:     true,
		Target:     cfg.Target,
	}

	resp, err := d.cli.ImageBuild(ctx, tar, opts)
	if err != nil {
		return fmt.Errorf("docker: building image: %w", err)
	}
	defer resp.Body.Close()

	// Stream the build output and check for errors.
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var msg struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &msg); err == nil && msg.Error != "" {
			return fmt.Errorf("docker: build error: %s", msg.Error)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("docker: reading build output: %w", err)
	}

	return nil
}

// RunContainer is a convenience method that creates, starts, and optionally
// waits for a container.
func (d *DockerClient) RunContainer(ctx context.Context, cfg ContainerConfig, wait bool) (string, int64, error) {
	id, err := d.CreateContainer(ctx, cfg)
	if err != nil {
		return "", -1, err
	}

	if err := d.StartContainer(ctx, id); err != nil {
		_ = d.RemoveContainer(ctx, id, true)
		return "", -1, err
	}

	if !wait {
		return id, 0, nil
	}

	exitCode, err := d.WaitForContainer(ctx, id)
	return id, exitCode, err
}
