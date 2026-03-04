package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Mscotello/docker-agent-tail/internal/docker"
)

func TestLogWriter(t *testing.T) {
	tests := []struct {
		name  string
		lines []docker.LogLine
	}{
		{
			name: "single container",
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
			name: "multiple containers",
			lines: []docker.LogLine{
				{
					Timestamp:     time.Now(),
					Stream:        "stdout",
					Content:       "App started",
					ContainerName: "app",
				},
				{
					Timestamp:     time.Now(),
					Stream:        "stderr",
					Content:       "DB ready",
					ContainerName: "database",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()

			writer, err := NewLogWriter(tmpDir)
			if err != nil {
				t.Fatalf("NewLogWriter() error = %v", err)
			}

			for _, line := range tt.lines {
				writer.Write(line)
			}

			if err := writer.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}

			// Check combined.jsonl exists and is valid JSONL
			combinedPath := filepath.Join(tmpDir, "combined.jsonl")
			combinedData, err := os.ReadFile(combinedPath)
			if err != nil {
				t.Fatalf("combined.jsonl not created: %v", err)
			}

			combinedLines := strings.Split(strings.TrimSpace(string(combinedData)), "\n")
			if len(combinedLines) != len(tt.lines) {
				t.Errorf("combined.jsonl has %d lines, want %d", len(combinedLines), len(tt.lines))
			}

			for i, cl := range combinedLines {
				var obj map[string]any
				if err := json.Unmarshal([]byte(cl), &obj); err != nil {
					t.Errorf("combined.jsonl line %d is not valid JSON: %v", i, err)
					continue
				}
				for _, key := range []string{"ts", "container", "stream"} {
					if _, ok := obj[key]; !ok {
						t.Errorf("combined.jsonl line %d missing key %q", i, key)
					}
				}
			}

			// Check per-container files exist and are valid JSONL
			for _, line := range tt.lines {
				containerLogPath := filepath.Join(tmpDir, line.ContainerName+".jsonl")
				data, err := os.ReadFile(containerLogPath)
				if err != nil {
					t.Fatalf("container log %s.jsonl not created: %v", line.ContainerName, err)
				}

				var obj map[string]any
				if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &obj); err != nil {
					t.Errorf("container log %s.jsonl is not valid JSON: %v", line.ContainerName, err)
					continue
				}

				if obj["container"] != line.ContainerName {
					t.Errorf("container = %v, want %q", obj["container"], line.ContainerName)
				}
				if obj["stream"] != line.Stream {
					t.Errorf("stream = %v, want %q", obj["stream"], line.Stream)
				}
			}
		})
	}
}

func TestLogWriter_WriteAfterClose(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	writer, err := NewLogWriter(tmpDir)
	if err != nil {
		t.Fatalf("NewLogWriter() error = %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Write after close should not panic
	writer.Write(docker.LogLine{
		Timestamp:     time.Now(),
		Stream:        "stdout",
		Content:       "should be dropped",
		ContainerName: "app",
	})
}

func TestLogWriter_WriteErrorLogging(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	writer, err := NewLogWriter(tmpDir)
	if err != nil {
		t.Fatalf("NewLogWriter() error = %v", err)
	}

	// Close the combined file to force write errors
	writer.combinedFile.Close()

	var mu sync.Mutex
	errors := make(map[string]int)
	writer.onWriteError = func(containerName string, err error) {
		mu.Lock()
		errors[containerName]++
		mu.Unlock()
	}

	// Write multiple lines — should only report error once per target
	for i := 0; i < 5; i++ {
		writer.Write(docker.LogLine{
			Timestamp:     time.Now(),
			Stream:        "stdout",
			Content:       "test",
			ContainerName: "app",
		})
	}

	// Wait for flusher to process
	writer.flushTicker.Stop()
	close(writer.done)
	writer.flusherWg.Wait()

	mu.Lock()
	combinedCount := errors["combined"]
	mu.Unlock()

	if combinedCount != 1 {
		t.Errorf("expected 1 error report for combined, got %d", combinedCount)
	}
}

func TestLogWriter_FlusherDrainsQueue(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	writer, err := NewLogWriter(tmpDir)
	if err != nil {
		t.Fatalf("NewLogWriter() error = %v", err)
	}

	// Write many lines concurrently
	const count = 100
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			writer.Write(docker.LogLine{
				Timestamp:     time.Now(),
				Stream:        "stdout",
				Content:       "test line",
				ContainerName: "app",
			})
		}()
	}
	wg.Wait()

	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// All lines should be in combined.jsonl
	data, err := os.ReadFile(filepath.Join(tmpDir, "combined.jsonl"))
	if err != nil {
		t.Fatalf("reading combined.jsonl: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != count {
		t.Errorf("combined.jsonl has %d lines, want %d", len(lines), count)
	}
}
