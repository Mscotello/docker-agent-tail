package docker

import (
	"context"
	"io"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
)

// MockDockerClient is a test mock for DockerClient.
type MockDockerClient struct {
	containers []container.Summary
	err        error
}

func (m *MockDockerClient) ContainerList(ctx context.Context, opts container.ListOptions) ([]container.Summary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.containers, nil
}

func (m *MockDockerClient) ContainerLogs(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (m *MockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return types.ContainerJSON{}, nil
}

func (m *MockDockerClient) Events(ctx context.Context, opts events.ListOptions) (<-chan events.Message, <-chan error) {
	ch := make(chan events.Message)
	errCh := make(chan error, 1)
	close(ch)
	return ch, errCh
}

func (m *MockDockerClient) Close() error {
	return nil
}

func TestDiscoverContainers(t *testing.T) {
	tests := []struct {
		name    string
		clients *MockDockerClient
		opts    DiscoverOpts
		want    int
	}{
		{
			name: "no filters",
			clients: &MockDockerClient{
				containers: []container.Summary{
					{Names: []string{"/app"}, Image: "app:latest", Labels: map[string]string{}},
					{Names: []string{"/db"}, Image: "db:latest", Labels: map[string]string{}},
				},
			},
			opts: DiscoverOpts{},
			want: 2,
		},
		{
			name: "glob pattern match",
			clients: &MockDockerClient{
				containers: []container.Summary{
					{Names: []string{"/app-1"}, Image: "app:latest", Labels: map[string]string{}},
					{Names: []string{"/app-2"}, Image: "app:latest", Labels: map[string]string{}},
					{Names: []string{"/db"}, Image: "db:latest", Labels: map[string]string{}},
				},
			},
			opts: DiscoverOpts{GlobPattern: "app-*"},
			want: 2,
		},
		{
			name: "compose project filter",
			clients: &MockDockerClient{
				containers: []container.Summary{
					{Names: []string{"/proj_app"}, Image: "app:latest", Labels: map[string]string{"com.docker.compose.project": "proj"}},
					{Names: []string{"/other_app"}, Image: "app:latest", Labels: map[string]string{"com.docker.compose.project": "other"}},
				},
			},
			opts: DiscoverOpts{ComposeProject: "proj"},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := DiscoverContainers(context.Background(), tt.clients, tt.opts)
			if err != nil {
				t.Fatalf("DiscoverContainers() error = %v", err)
			}
			if len(got) != tt.want {
				t.Errorf("DiscoverContainers() got %d containers, want %d", len(got), tt.want)
			}
		})
	}
}
