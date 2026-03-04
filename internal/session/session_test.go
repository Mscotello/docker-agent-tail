package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNewSession(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		containers []string
	}{
		{
			name:       "basic session",
			command:    "docker-agent-tail --all",
			containers: []string{"app", "db"},
		},
		{
			name:       "empty containers",
			command:    "docker-agent-tail",
			containers: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			sess, err := NewSession(tmpDir, tt.command, tt.containers)
			if err != nil {
				t.Fatalf("NewSession() error = %v", err)
			}

			// Check session directory exists
			if _, err := os.Stat(sess.Dir); os.IsNotExist(err) {
				t.Errorf("session directory not created")
			}

			// Check metadata.json exists and is correct
			metadataPath := filepath.Join(sess.Dir, "metadata.json")
			if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
				t.Errorf("metadata.json not created")
			}

			data, err := os.ReadFile(metadataPath)
			if err != nil {
				t.Fatalf("reading metadata: %v", err)
			}

			var meta Metadata
			if err := json.Unmarshal(data, &meta); err != nil {
				t.Fatalf("unmarshaling metadata: %v", err)
			}

			if meta.Command != tt.command {
				t.Errorf("command mismatch: got %q, want %q", meta.Command, tt.command)
			}
			if len(meta.Containers) != len(tt.containers) {
				t.Errorf("containers mismatch: got %d, want %d", len(meta.Containers), len(tt.containers))
			}

			// Check latest symlink points to correct session
			latestLink := filepath.Join(tmpDir, "latest")
			if _, err := os.Lstat(latestLink); os.IsNotExist(err) {
				t.Errorf("latest symlink not created")
			}
			target, err := os.Readlink(latestLink)
			if err != nil {
				t.Fatalf("reading latest symlink: %v", err)
			}
			wantTarget := sess.StartTime.Format("2006-01-02-150405")
			if target != wantTarget {
				t.Errorf("latest symlink = %q, want %q", target, wantTarget)
			}
		})
	}
}

func TestSession_ConcurrentContainerAppend(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	sess, err := NewSession(tmpDir, "cmd", []string{"initial"})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			sess.ContainersMu.Lock()
			sess.Containers = append(sess.Containers, fmt.Sprintf("container-%d", n))
			sess.ContainersMu.Unlock()
		}(i)
	}
	wg.Wait()

	sess.ContainersMu.Lock()
	got := len(sess.Containers)
	sess.ContainersMu.Unlock()

	want := 1 + goroutines // "initial" + goroutines
	if got != want {
		t.Errorf("Containers length = %d, want %d", got, want)
	}
}

func TestNewSession_symlink_update(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create first session
	sess1, err := NewSession(tmpDir, "cmd1", []string{"a"})
	if err != nil {
		t.Fatalf("first NewSession() error = %v", err)
	}

	// Verify symlink points to first session
	latestLink := filepath.Join(tmpDir, "latest")
	target1, err := os.Readlink(latestLink)
	if err != nil {
		t.Fatalf("reading latest symlink: %v", err)
	}
	if target1 != filepath.Base(sess1.Dir) {
		t.Errorf("latest = %q, want %q", target1, filepath.Base(sess1.Dir))
	}
}
