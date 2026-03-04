// Package docker provides Docker SDK client operations.
package docker

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// ContainerInfo holds information about a discovered container.
type ContainerInfo struct {
	Name   string
	ID     string
	Image  string
	Labels map[string]string
}

// DiscoverOpts configures container discovery.
type DiscoverOpts struct {
	// GlobPattern filters container names (empty = all)
	GlobPattern string
	// ComposeProject filters by Docker Compose project label (empty = ignore)
	ComposeProject string
}

// DiscoverContainers lists running containers matching the given filters.
func DiscoverContainers(ctx context.Context, c DockerClient, opts DiscoverOpts) ([]ContainerInfo, error) {
	containers, err := c.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	var result []ContainerInfo
	for _, ctr := range containers {
		// Remove leading slash from container name
		name := strings.TrimPrefix(ctr.Names[0], "/")

		// Filter by glob pattern if specified
		if opts.GlobPattern != "" {
			matched, err := filepath.Match(opts.GlobPattern, name)
			if err != nil || !matched {
				continue
			}
		}

		// Filter by compose project if specified
		if opts.ComposeProject != "" {
			project, ok := ctr.Labels["com.docker.compose.project"]
			if !ok || project != opts.ComposeProject {
				continue
			}
		}

		result = append(result, ContainerInfo{
			Name:   name,
			ID:     ctr.ID,
			Image:  ctr.Image,
			Labels: ctr.Labels,
		})
	}

	return result, nil
}
