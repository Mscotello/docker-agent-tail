package output

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Mscotello/docker-agent-tail/internal/docker"
)

func TestNewOutputWriter(t *testing.T) {
	tests := []struct {
		name           string
		noColor        bool
		muteContainers []string
	}{
		{
			name:           "with color",
			noColor:        false,
			muteContainers: []string{},
		},
		{
			name:           "no color",
			noColor:        true,
			muteContainers: []string{},
		},
		{
			name:           "mute containers",
			noColor:        false,
			muteContainers: []string{"app", "db"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			ow := NewOutputWriter(buf, tt.noColor, tt.muteContainers)

			if ow == nil {
				t.Errorf("NewOutputWriter() returned nil")
			}
		})
	}
}

func TestWriteLogLine(t *testing.T) {
	tests := []struct {
		name           string
		noColor        bool
		muteContainers []string
		line           docker.LogLine
		expectOutput   bool
	}{
		{
			name:           "normal output",
			noColor:        true,
			muteContainers: []string{},
			line: docker.LogLine{
				Timestamp:     time.Now(),
				Stream:        "stdout",
				Content:       "test message",
				ContainerName: "app",
			},
			expectOutput: true,
		},
		{
			name:           "muted container",
			noColor:        true,
			muteContainers: []string{"app"},
			line: docker.LogLine{
				Timestamp:     time.Now(),
				Stream:        "stdout",
				Content:       "test message",
				ContainerName: "app",
			},
			expectOutput: false,
		},
		{
			name:           "stderr output",
			noColor:        true,
			muteContainers: []string{},
			line: docker.LogLine{
				Timestamp:     time.Now(),
				Stream:        "stderr",
				Content:       "error message",
				ContainerName: "app",
			},
			expectOutput: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			ow := NewOutputWriter(buf, tt.noColor, tt.muteContainers)

			ow.WriteLogLine(tt.line)

			output := buf.String()
			if tt.expectOutput && output == "" {
				t.Errorf("WriteLogLine() expected output but got nothing")
			}
			if !tt.expectOutput && output != "" {
				t.Errorf("WriteLogLine() expected no output but got: %q", output)
			}

			if tt.expectOutput {
				if !strings.Contains(output, tt.line.Content) {
					t.Errorf("output missing content: %q", tt.line.Content)
				}
				if !strings.Contains(output, tt.line.ContainerName) {
					t.Errorf("output missing container name: %q", tt.line.ContainerName)
				}
				if !strings.Contains(output, tt.line.Stream) {
					t.Errorf("output missing stream: %q", tt.line.Stream)
				}
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name           string
		muteContainers []string
		line           docker.LogLine
		expectOutput   bool
	}{
		{
			name:           "normal json output",
			muteContainers: []string{},
			line: docker.LogLine{
				Timestamp:     time.Now(),
				Stream:        "stdout",
				Content:       "test message",
				ContainerName: "app",
			},
			expectOutput: true,
		},
		{
			name:           "muted container json",
			muteContainers: []string{"app"},
			line: docker.LogLine{
				Timestamp:     time.Now(),
				Stream:        "stdout",
				Content:       "test message",
				ContainerName: "app",
			},
			expectOutput: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			ow := NewOutputWriter(buf, true, tt.muteContainers)

			err := ow.WriteJSON(tt.line)
			if err != nil {
				t.Fatalf("WriteJSON() error = %v", err)
			}

			output := buf.String()
			if tt.expectOutput && output == "" {
				t.Errorf("WriteJSON() expected output but got nothing")
			}
			if !tt.expectOutput && output != "" {
				t.Errorf("WriteJSON() expected no output but got: %q", output)
			}

			if tt.expectOutput {
				if !strings.Contains(output, tt.line.ContainerName) {
					t.Errorf("JSON output missing container: %q", tt.line.ContainerName)
				}
				if !strings.Contains(output, tt.line.Content) {
					t.Errorf("JSON output missing content: %q", tt.line.Content)
				}
				if !strings.Contains(output, "timestamp") {
					t.Errorf("JSON output missing timestamp")
				}
			}
		})
	}
}

func TestWriteLogLine_JSONMode(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	ow := NewOutputWriter(buf, false, nil, true)

	line := docker.LogLine{
		Timestamp:     time.Date(2026, 3, 4, 10, 30, 1, 0, time.UTC),
		Stream:        "stdout",
		Content:       "test message",
		ContainerName: "api",
	}
	ow.WriteLogLine(line)

	output := buf.String()
	if !strings.Contains(output, `"container":"api"`) {
		t.Errorf("JSON output missing container field: %s", output)
	}
	if !strings.Contains(output, `"stream":"stdout"`) {
		t.Errorf("JSON output missing stream field: %s", output)
	}
	if !strings.Contains(output, `"message"`) {
		t.Errorf("JSON output missing message field: %s", output)
	}
	// Should NOT contain color-formatted output
	if strings.Contains(output, "[") && strings.Contains(output, "]") && !strings.Contains(output, `"`) {
		t.Errorf("JSON mode should not produce human-readable format")
	}
}

func TestNoColorEnv(t *testing.T) {
	// Test NO_COLOR environment variable
	old := os.Getenv("NO_COLOR")
	defer func() {
		if old != "" {
			os.Setenv("NO_COLOR", old)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	os.Setenv("NO_COLOR", "1")
	buf := &bytes.Buffer{}
	ow := NewOutputWriter(buf, false, []string{})

	if !ow.noColor {
		t.Errorf("NO_COLOR env var not respected")
	}
}
