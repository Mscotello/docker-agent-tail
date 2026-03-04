package session

import (
	"encoding/json"
	"os"
	"path/filepath"
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

			// Check latest symlink
			latestLink := filepath.Join(tmpDir, "latest")
			if _, err := os.Lstat(latestLink); os.IsNotExist(err) {
				t.Errorf("latest symlink not created")
			}
		})
	}
}
