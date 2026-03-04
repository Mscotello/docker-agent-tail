package session

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Mscotello/docker-agent-tail/internal/docker"
)

func TestFormatJSONL(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 3, 4, 10, 30, 1, 789000000, time.UTC)
	baseLine := docker.LogLine{
		Timestamp:     ts,
		Stream:        "stdout",
		ContainerName: "api",
	}

	tests := []struct {
		name        string
		content     string
		wantKeys    []string
		wantMessage bool   // true if "message" key should exist
		wantMsgVal  string // expected message value (only checked if wantMessage)
	}{
		{
			name:        "plain text",
			content:     "GET /api/users 200",
			wantKeys:    []string{"ts", "container", "stream", "message"},
			wantMessage: true,
			wantMsgVal:  "GET /api/users 200",
		},
		{
			name:     "json object merged",
			content:  `{"level":"info","msg":"started"}`,
			wantKeys: []string{"ts", "container", "stream", "level", "msg"},
		},
		{
			name:        "json array wrapped",
			content:     `[1,2,3]`,
			wantKeys:    []string{"ts", "container", "stream", "message"},
			wantMessage: true,
			wantMsgVal:  "[1,2,3]",
		},
		{
			name:        "json string wrapped",
			content:     `"hello"`,
			wantKeys:    []string{"ts", "container", "stream", "message"},
			wantMessage: true,
			wantMsgVal:  `"hello"`,
		},
		{
			name:        "malformed json wrapped",
			content:     `{"broken":`,
			wantKeys:    []string{"ts", "container", "stream", "message"},
			wantMessage: true,
			wantMsgVal:  `{"broken":`,
		},
		{
			name:     "metadata collision overwritten",
			content:  `{"ts":"old","stream":"old","custom":"value"}`,
			wantKeys: []string{"ts", "container", "stream", "custom"},
		},
		{
			name:        "empty content",
			content:     "",
			wantKeys:    []string{"ts", "container", "stream", "message"},
			wantMessage: true,
			wantMsgVal:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			line := baseLine
			line.Content = tt.content

			got := FormatJSONL(line)

			// Must end with newline
			if len(got) == 0 || got[len(got)-1] != '\n' {
				t.Fatal("output must end with newline")
			}

			// Must be valid JSON
			var obj map[string]any
			if err := json.Unmarshal(got, &obj); err != nil {
				t.Fatalf("invalid JSON: %v\noutput: %s", err, got)
			}

			// Check required keys exist
			for _, key := range tt.wantKeys {
				if _, ok := obj[key]; !ok {
					t.Errorf("missing key %q in output: %s", key, got)
				}
			}

			// Verify metadata values
			if obj["ts"] != ts.UTC().Format(time.RFC3339Nano) {
				t.Errorf("ts = %v, want %v", obj["ts"], ts.UTC().Format(time.RFC3339Nano))
			}
			if obj["container"] != "api" {
				t.Errorf("container = %v, want %q", obj["container"], "api")
			}
			if obj["stream"] != "stdout" {
				t.Errorf("stream = %v, want %q", obj["stream"], "stdout")
			}

			// Check message field
			if tt.wantMessage {
				msg, ok := obj["message"]
				if !ok {
					t.Fatal("missing message key")
				}
				if msg != tt.wantMsgVal {
					t.Errorf("message = %q, want %q", msg, tt.wantMsgVal)
				}
			}

			// For collision test: verify metadata was overwritten
			if tt.name == "metadata collision overwritten" {
				if obj["ts"] == "old" {
					t.Error("ts should be overwritten, got 'old'")
				}
				if obj["stream"] == "old" {
					t.Error("stream should be overwritten, got 'old'")
				}
			}
		})
	}
}
