package docker

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
)

func TestParseContainerEvent(t *testing.T) {
	tests := []struct {
		name     string
		msg      events.Message
		opts     WatchEventsOpts
		wantOK   bool
		wantType EventType
	}{
		{
			name: "start event with container type",
			msg: events.Message{
				Type:   events.ContainerEventType,
				Action: "start",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name": "myapp",
					},
				},
				TimeNano: 1234567890000000000,
			},
			opts:     WatchEventsOpts{},
			wantOK:   true,
			wantType: EventStart,
		},
		{
			name: "stop event",
			msg: events.Message{
				Type:   events.ContainerEventType,
				Action: "stop",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name": "myapp",
					},
				},
			},
			opts:     WatchEventsOpts{},
			wantOK:   true,
			wantType: EventStop,
		},
		{
			name: "die event",
			msg: events.Message{
				Type:   events.ContainerEventType,
				Action: "die",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name": "myapp",
					},
				},
			},
			opts:     WatchEventsOpts{},
			wantOK:   true,
			wantType: EventDie,
		},
		{
			name: "restart event",
			msg: events.Message{
				Type:   events.ContainerEventType,
				Action: "restart",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name": "myapp",
					},
				},
			},
			opts:     WatchEventsOpts{},
			wantOK:   true,
			wantType: EventRestart,
		},
		{
			name: "non-container event type",
			msg: events.Message{
				Type:   events.ImageEventType,
				Action: "build",
				Actor: events.Actor{
					ID: "abc123",
				},
			},
			opts:   WatchEventsOpts{},
			wantOK: false,
		},
		{
			name: "unsupported action",
			msg: events.Message{
				Type:   events.ContainerEventType,
				Action: "create",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name": "myapp",
					},
				},
			},
			opts:   WatchEventsOpts{},
			wantOK: false,
		},
		{
			name: "glob pattern match",
			msg: events.Message{
				Type:   events.ContainerEventType,
				Action: "start",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name": "app-1",
					},
				},
			},
			opts:     WatchEventsOpts{GlobPattern: "app-*"},
			wantOK:   true,
			wantType: EventStart,
		},
		{
			name: "glob pattern no match",
			msg: events.Message{
				Type:   events.ContainerEventType,
				Action: "start",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name": "db-1",
					},
				},
			},
			opts:   WatchEventsOpts{GlobPattern: "app-*"},
			wantOK: false,
		},
		{
			name: "compose project filter match",
			msg: events.Message{
				Type:   events.ContainerEventType,
				Action: "start",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name":                       "proj_app",
						"com.docker.compose.project": "proj",
					},
				},
			},
			opts:     WatchEventsOpts{ComposeProject: "proj"},
			wantOK:   true,
			wantType: EventStart,
		},
		{
			name: "compose project filter no match",
			msg: events.Message{
				Type:   events.ContainerEventType,
				Action: "start",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name":                       "other_app",
						"com.docker.compose.project": "other",
					},
				},
			},
			opts:   WatchEventsOpts{ComposeProject: "proj"},
			wantOK: false,
		},
		{
			name: "missing container name",
			msg: events.Message{
				Type:   events.ContainerEventType,
				Action: "start",
				Actor: events.Actor{
					ID:         "abc123",
					Attributes: map[string]string{},
				},
			},
			opts:     WatchEventsOpts{},
			wantOK:   true, // name will be empty string
			wantType: EventStart,
		},
		{
			name: "empty actor ID",
			msg: events.Message{
				Type:   events.ContainerEventType,
				Action: "start",
				Actor: events.Actor{
					ID:         "",
					Attributes: map[string]string{"name": "app"},
				},
			},
			opts:   WatchEventsOpts{},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			event, ok := parseContainerEvent(tt.msg, tt.opts)
			if ok != tt.wantOK {
				t.Errorf("parseContainerEvent() ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if ok && event.Type != tt.wantType {
				t.Errorf("parseContainerEvent() Type = %v, want %v", event.Type, tt.wantType)
			}
			if ok && event.ContainerID != tt.msg.Actor.ID {
				t.Errorf("parseContainerEvent() ContainerID = %s, want %s", event.ContainerID, tt.msg.Actor.ID)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		input     string
		wantMatch bool
	}{
		{
			name:      "empty pattern matches all",
			pattern:   "",
			input:     "anything",
			wantMatch: true,
		},
		{
			name:      "exact match",
			pattern:   "app",
			input:     "app",
			wantMatch: true,
		},
		{
			name:      "exact no match",
			pattern:   "app",
			input:     "app-1",
			wantMatch: false,
		},
		{
			name:      "prefix wildcard",
			pattern:   "app-*",
			input:     "app-1",
			wantMatch: true,
		},
		{
			name:      "prefix wildcard no match",
			pattern:   "app-*",
			input:     "db-1",
			wantMatch: false,
		},
		{
			name:      "suffix wildcard",
			pattern:   "*-app",
			input:     "my-app",
			wantMatch: true,
		},
		{
			name:      "suffix wildcard no match",
			pattern:   "*-app",
			input:     "app-my",
			wantMatch: false,
		},
		{
			name:      "middle wildcard",
			pattern:   "app*service",
			input:     "appv1service",
			wantMatch: true,
		},
		{
			name:      "multiple wildcards",
			pattern:   "app*-*service",
			input:     "appv1-testservice",
			wantMatch: true,
		},
		{
			name:      "multiple wildcards no match",
			pattern:   "app*-*service",
			input:     "appv1testservice",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := matchesPattern(tt.input, tt.pattern)
			if got != tt.wantMatch {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.input, tt.pattern, got, tt.wantMatch)
			}
		})
	}
}

// MockDockerClientForEvents is a test mock that can emit events.
type MockDockerClientForEvents struct {
	eventsCh chan events.Message
	errsCh   chan error
}

func (m *MockDockerClientForEvents) ContainerList(ctx context.Context, opts container.ListOptions) ([]container.Summary, error) {
	return nil, nil
}

func (m *MockDockerClientForEvents) ContainerLogs(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (m *MockDockerClientForEvents) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return types.ContainerJSON{}, nil
}

func (m *MockDockerClientForEvents) Events(ctx context.Context, opts events.ListOptions) (<-chan events.Message, <-chan error) {
	return m.eventsCh, m.errsCh
}

func (m *MockDockerClientForEvents) Close() error {
	return nil
}

func TestWatchEvents(t *testing.T) {
	tests := []struct {
		name      string
		events    []events.Message
		opts      WatchEventsOpts
		wantCount int
	}{
		{
			name: "single start event",
			events: []events.Message{
				{
					Type:     events.ContainerEventType,
					Action:   "start",
					TimeNano: 1234567890000000000,
					Actor: events.Actor{
						ID:         "abc123",
						Attributes: map[string]string{"name": "app"},
					},
				},
			},
			opts:      WatchEventsOpts{},
			wantCount: 1,
		},
		{
			name: "multiple events filtered",
			events: []events.Message{
				{
					Type:   events.ContainerEventType,
					Action: "start",
					Actor: events.Actor{
						ID:         "abc123",
						Attributes: map[string]string{"name": "app-1"},
					},
				},
				{
					Type:   events.ContainerEventType,
					Action: "start",
					Actor: events.Actor{
						ID:         "def456",
						Attributes: map[string]string{"name": "db-1"},
					},
				},
				{
					Type:   events.ContainerEventType,
					Action: "stop",
					Actor: events.Actor{
						ID:         "abc123",
						Attributes: map[string]string{"name": "app-1"},
					},
				},
			},
			opts:      WatchEventsOpts{GlobPattern: "app-*"},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			mock := &MockDockerClientForEvents{
				eventsCh: make(chan events.Message),
				errsCh:   make(chan error, 1),
			}

			eventCh, _ := WatchEvents(ctx, mock, tt.opts)

			go func() {
				for _, e := range tt.events {
					mock.eventsCh <- e
				}
				close(mock.eventsCh)
			}()

			count := 0
			for {
				select {
				case event := <-eventCh:
					if event.ContainerName == "" && event.ContainerID == "" {
						t.Errorf("got empty event")
						continue
					}
					count++
				case <-ctx.Done():
					goto done
				}
			}
		done:
			if count != tt.wantCount {
				t.Errorf("WatchEvents got %d events, want %d", count, tt.wantCount)
			}
		})
	}
}
