package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/scotello/docker-agent-tail/internal/docker"
)

func TestLogWriter(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		maxNameLen int
		lines      []docker.LogLine
	}{
		{
			name:       "single container",
			maxNameLen: 5,
			lines: []docker.LogLine{
				{
					Timestamp:     time.Now(),
					Stream:        "stdout",
					Content:       "Hello world",
					ContainerName: "app",
				},
			},
		},
		{
			name:       "multiple containers",
			maxNameLen: 6,
			lines: []docker.LogLine{
				{
					Timestamp:     time.Now(),
					Stream:        "stdout",
					Content:       "App started",
					ContainerName: "app",
				},
				{
					Timestamp:     time.Now(),
					Stream:        "stdout",
					Content:       "DB ready",
					ContainerName: "database",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			writer, err := NewLogWriter(tmpDir, tt.maxNameLen)
			if err != nil {
				t.Fatalf("NewLogWriter() error = %v", err)
			}

			for _, line := range tt.lines {
				writer.Write(line)
			}

			if err := writer.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}

			// Check combined.log exists
			combinedPath := filepath.Join(tmpDir, "combined.log")
			if _, err := os.Stat(combinedPath); os.IsNotExist(err) {
				t.Errorf("combined.log not created")
			}

			// Check per-container files exist
			for _, line := range tt.lines {
				containerLogPath := filepath.Join(tmpDir, line.ContainerName+".log")
				if _, err := os.Stat(containerLogPath); os.IsNotExist(err) {
					t.Errorf("container log %s not created", line.ContainerName+".log")
				}

				// Check content
				content, err := os.ReadFile(containerLogPath)
				if err != nil {
					t.Fatalf("reading container log: %v", err)
				}

				if !strings.Contains(string(content), line.Content) {
					t.Errorf("container log missing content: %q", line.Content)
				}
			}
		})
	}
}

func TestFormatLogLine(t *testing.T) {
	now := time.Now()
	line := docker.LogLine{
		Timestamp:     now,
		Stream:        "stdout",
		Content:       "test message",
		ContainerName: "app",
	}

	formatted := formatLogLine(line, false)

	if !strings.Contains(formatted, "stdout") {
		t.Errorf("formatted line missing stream type")
	}
	if !strings.Contains(formatted, "test message") {
		t.Errorf("formatted line missing content")
	}
}

func TestFormatCombinedLine(t *testing.T) {
	now := time.Now()
	line := docker.LogLine{
		Timestamp:     now,
		Stream:        "stdout",
		Content:       "test message",
		ContainerName: "app",
	}

	formatted := formatCombinedLine(line, 10)

	if !strings.Contains(formatted, "app") {
		t.Errorf("combined line missing container name")
	}
	if !strings.Contains(formatted, "stdout") {
		t.Errorf("combined line missing stream type")
	}
	if !strings.Contains(formatted, "test message") {
		t.Errorf("combined line missing content")
	}
}
