// Package docker provides Docker SDK client operations.
package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/events"
)

// EventType represents a Docker container event type.
type EventType string

const (
	EventStart   EventType = "start"
	EventStop    EventType = "stop"
	EventDie     EventType = "die"
	EventRestart EventType = "restart"
)

// ContainerEvent represents a Docker container lifecycle event.
type ContainerEvent struct {
	Type          EventType
	ContainerID   string
	ContainerName string
	Time          time.Time
}

// WatchEventsOpts configures event watching.
type WatchEventsOpts struct {
	// GlobPattern filters container names (empty = all)
	GlobPattern string
	// ComposeProject filters by Docker Compose project label (empty = ignore)
	ComposeProject string
}

// WatchEvents subscribes to Docker container events and streams them via a channel.
// It filters for start, stop, die, and restart events on containers matching the given criteria.
func WatchEvents(ctx context.Context, c DockerClient, opts WatchEventsOpts) (<-chan ContainerEvent, <-chan error) {
	eventCh := make(chan ContainerEvent)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)

		// Subscribe to Docker events
		messages, errs := c.Events(ctx, events.ListOptions{})

		for {
			select {
			case <-ctx.Done():
				return
			case err := <-errs:
				if err != nil {
					errCh <- fmt.Errorf("watching events: %w", err)
				}
				return
			case msg := <-messages:
				event, ok := parseContainerEvent(msg, opts)
				if ok {
					eventCh <- event
				}
			}
		}
	}()

	return eventCh, errCh
}

// parseContainerEvent extracts container event information from a Docker event message.
// Returns the parsed event and a boolean indicating whether the event should be processed.
func parseContainerEvent(msg events.Message, opts WatchEventsOpts) (ContainerEvent, bool) {
	// Only process container events
	if msg.Type != events.ContainerEventType {
		return ContainerEvent{}, false
	}

	// Get container name from Actor
	if msg.Actor.ID == "" {
		return ContainerEvent{}, false
	}

	containerID := msg.Actor.ID
	containerName := msg.Actor.Attributes["name"]

	// Filter by glob pattern if specified
	if opts.GlobPattern != "" {
		// Use filepath.Match-like logic (already available in discover.go)
		// For now, we'll do a simple match
		matched := matchesPattern(containerName, opts.GlobPattern)
		if !matched {
			return ContainerEvent{}, false
		}
	}

	// Filter by compose project if specified
	if opts.ComposeProject != "" {
		project := msg.Actor.Attributes["com.docker.compose.project"]
		if project != opts.ComposeProject {
			return ContainerEvent{}, false
		}
	}

	// Map event action to our EventType
	var eventType EventType
	switch msg.Action {
	case "start":
		eventType = EventStart
	case "stop":
		eventType = EventStop
	case "die":
		eventType = EventDie
	case "restart":
		eventType = EventRestart
	default:
		return ContainerEvent{}, false
	}

	return ContainerEvent{
		Type:          eventType,
		ContainerID:   containerID,
		ContainerName: containerName,
		Time:          time.Unix(0, msg.TimeNano),
	}, true
}

// matchesPattern does simple glob matching for container names.
// Supports * wildcard.
func matchesPattern(name, pattern string) bool {
	if pattern == "" {
		return true
	}

	// Simple glob matching
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		// No wildcards
		return name == pattern
	}

	idx := 0
	for i, part := range parts {
		if i == 0 {
			// First part must match at the beginning
			if part != "" && !strings.HasPrefix(name, part) {
				return false
			}
			idx = len(part)
		} else if i == len(parts)-1 {
			// Last part must match at the end
			if part != "" && !strings.HasSuffix(name, part) {
				return false
			}
		} else {
			// Middle parts must be found in order
			if part == "" {
				continue
			}
			pos := strings.Index(name[idx:], part)
			if pos == -1 {
				return false
			}
			idx += pos + len(part)
		}
	}

	return true
}
