package docker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDeployer(t *testing.T) {
	d, err := NewDeployer()
	require.NoError(t, err)
	assert.NotNil(t, d)
	assert.NotNil(t, d.client)
	assert.NotNil(t, d.deployments)
}

func TestDeployConfig(t *testing.T) {
	d, err := NewDeployer()
	require.NoError(t, err)

	ctx := context.Background()

	// Nil config should return an error.
	_, err = d.Deploy(ctx, nil)
	assert.Error(t, err)

	// Empty image should return an error.
	_, err = d.Deploy(ctx, &DeployConfig{Image: ""})
	assert.Error(t, err)

	// Valid config deploys successfully.
	config := &DeployConfig{
		ProjectName: "test-project",
		Image:       "nginx",
		Tag:         "latest",
		Port:        8080,
		Replicas:    2,
		Env:         map[string]string{"ENV": "test"},
		Network:     "test-net",
	}

	dep, err := d.Deploy(ctx, config)
	require.NoError(t, err)
	assert.NotNil(t, dep)
	assert.Equal(t, "test-project", dep.ProjectName)
	assert.Equal(t, "nginx:latest", dep.Image)
	assert.Equal(t, "running", dep.Status)
	assert.Len(t, dep.Containers, 2)
	assert.False(t, dep.CreatedAt.IsZero())

	// Verify container info.
	for i, c := range dep.Containers {
		assert.NotEmpty(t, c.ID)
		assert.Equal(t, "running", c.Status)
		assert.Equal(t, 8080+i, c.Port)
	}
}

func TestDeployConfigDefaultReplicas(t *testing.T) {
	d, err := NewDeployer()
	require.NoError(t, err)

	ctx := context.Background()
	config := &DeployConfig{
		Image: "alpine",
		Port:  3000,
		// Replicas is 0, should default to 1.
	}

	dep, err := d.Deploy(ctx, config)
	require.NoError(t, err)
	assert.Len(t, dep.Containers, 1)
}

func TestDeploymentStatus(t *testing.T) {
	d, err := NewDeployer()
	require.NoError(t, err)

	ctx := context.Background()

	// Deploy a service.
	config := &DeployConfig{
		ProjectName: "status-test",
		Image:       "busybox",
		Tag:         "latest",
		Port:        9090,
		Replicas:    1,
	}
	dep, err := d.Deploy(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "running", dep.Status)

	// Get status by ID.
	status, err := d.GetStatus(ctx, dep.ID)
	require.NoError(t, err)
	assert.Equal(t, "running", status.Status)
	assert.Equal(t, dep.ID, status.ID)

	// Stop the deployment.
	err = d.Stop(ctx, dep.ID)
	require.NoError(t, err)

	status, err = d.GetStatus(ctx, dep.ID)
	require.NoError(t, err)
	assert.Equal(t, "stopped", status.Status)

	// Nonexistent deployment.
	_, err = d.GetStatus(ctx, "nonexistent-id")
	assert.Error(t, err)
}
