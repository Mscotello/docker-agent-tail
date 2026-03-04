package docker

import (
	"strings"
	"testing"
	"time"
)

func TestParseLogLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		line          string
		containerName string
		stream        string
		wantContent   string
		wantStream    string
		wantErr       bool
	}{
		{
			name:          "stdout with content",
			line:          "2026-03-04T10:30:01.789Z GET /api/users 200 12ms",
			containerName: "api",
			stream:        "stdout",
			wantContent:   "GET /api/users 200 12ms",
			wantStream:    "stdout",
		},
		{
			name:          "stderr stream passed through",
			line:          "2026-03-04T10:30:01.789Z WARN: connection pool exhausted",
			containerName: "api",
			stream:        "stderr",
			wantContent:   "WARN: connection pool exhausted",
			wantStream:    "stderr",
		},
		{
			name:          "timestamp only no content",
			line:          "2026-03-04T10:30:01.789Z",
			containerName: "worker",
			stream:        "stdout",
			wantContent:   "",
			wantStream:    "stdout",
		},
		{
			name:          "nanosecond precision timestamp",
			line:          "2026-03-04T10:30:01.789123456Z some log",
			containerName: "app",
			stream:        "stdout",
			wantContent:   "some log",
			wantStream:    "stdout",
		},
		{
			name:          "invalid timestamp",
			line:          "not-a-timestamp some content",
			containerName: "api",
			stream:        "stdout",
			wantErr:       true,
		},
		{
			name:          "empty line",
			line:          "",
			containerName: "api",
			stream:        "stdout",
			wantErr:       true,
		},
		{
			name:          "content with spaces",
			line:          "2026-03-04T10:30:01.789Z  multiple  spaces  here",
			containerName: "api",
			stream:        "stdout",
			wantContent:   " multiple  spaces  here",
			wantStream:    "stdout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseLogLine(tt.line, tt.containerName, tt.stream)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseLogLine() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if got.Stream != tt.wantStream {
				t.Errorf("Stream = %q, want %q", got.Stream, tt.wantStream)
			}
			if got.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", got.Content, tt.wantContent)
			}
			if got.ContainerName != tt.containerName {
				t.Errorf("ContainerName = %q, want %q", got.ContainerName, tt.containerName)
			}
			if got.Timestamp.IsZero() {
				t.Error("Timestamp should not be zero")
			}
		})
	}
}

func TestReadStream(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 3, 4, 10, 30, 1, 789000000, time.UTC)

	tests := []struct {
		name       string
		input      string
		stream     string
		wantCount  int
		wantStream string
	}{
		{
			name:       "stdout lines",
			input:      ts.Format(time.RFC3339Nano) + " line one\n" + ts.Format(time.RFC3339Nano) + " line two\n",
			stream:     "stdout",
			wantCount:  2,
			wantStream: "stdout",
		},
		{
			name:       "stderr lines",
			input:      ts.Format(time.RFC3339Nano) + " error msg\n",
			stream:     "stderr",
			wantCount:  1,
			wantStream: "stderr",
		},
		{
			name:      "empty input",
			input:     "",
			stream:    "stdout",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logCh := make(chan LogLine, 10)
			errCh := make(chan error, 1)
			ctx := t.Context()

			reader := strings.NewReader(tt.input)
			readStream(ctx, reader, logCh, errCh, "test", tt.stream)
			close(logCh)

			var lines []LogLine
			for l := range logCh {
				lines = append(lines, l)
			}

			if len(lines) != tt.wantCount {
				t.Fatalf("got %d lines, want %d", len(lines), tt.wantCount)
			}

			for _, l := range lines {
				if l.Stream != tt.wantStream {
					t.Errorf("Stream = %q, want %q", l.Stream, tt.wantStream)
				}
			}
		})
	}
}
