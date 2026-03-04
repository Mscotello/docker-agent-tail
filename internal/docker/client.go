// Package docker provides Docker SDK client operations.
package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

// DockerClient defines the interface for Docker SDK operations used by this tool.
// This allows for mocking in tests.
type DockerClient interface {
	ContainerList(ctx context.Context, opts container.ListOptions) ([]container.Summary, error)
	ContainerLogs(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	Events(ctx context.Context, opts events.ListOptions) (<-chan events.Message, <-chan error)
	Close() error
}

// NewClient creates a Docker client with socket discovery.
// It tries sockets in this order:
// 1. $DOCKER_HOST environment variable
// 2. ~/.docker/run/docker.sock
// 3. /var/run/docker.sock
func NewClient(ctx context.Context) (DockerClient, error) {
	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}

	c, err := client.NewClientWithOpts(opts...)
	if err == nil {
		return c, nil
	}

	// Try ~/.docker/run/docker.sock
	home, err := os.UserHomeDir()
	if err == nil {
		dockerSock := filepath.Join(home, ".docker", "run", "docker.sock")
		if _, err := os.Stat(dockerSock); err == nil {
			c, err := client.NewClientWithOpts(
				client.WithHost(fmt.Sprintf("unix://%s", dockerSock)),
				client.WithAPIVersionNegotiation(),
			)
			if err == nil {
				return c, nil
			}
		}
	}

	// Try /var/run/docker.sock
	c, err = client.NewClientWithOpts(
		client.WithHost("unix:///var/run/docker.sock"),
		client.WithAPIVersionNegotiation(),
	)
	if err == nil {
		return c, nil
	}

	return nil, fmt.Errorf("discovering Docker socket: all strategies failed")
}
