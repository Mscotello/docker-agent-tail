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

func TestFormatJSONLLevelNormalization(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 3, 4, 10, 30, 1, 789000000, time.UTC)
	baseLine := docker.LogLine{
		Timestamp:     ts,
		Stream:        "stdout",
		ContainerName: "api",
	}

	tests := []struct {
		name       string
		content    string
		wantLevel  string // expected "level" value; empty means no level key
		absentKeys []string
	}{
		{
			name:      "level INFO normalized",
			content:   `{"level":"INFO","msg":"started"}`,
			wantLevel: "info",
		},
		{
			name:       "lvl alias removed",
			content:    `{"lvl":"warn","msg":"slow"}`,
			wantLevel:  "warning",
			absentKeys: []string{"lvl"},
		},
		{
			name:       "s alias for MongoDB",
			content:    `{"s":"I","c":"CONTROL","msg":"starting"}`,
			wantLevel:  "info",
			absentKeys: []string{"s"},
		},
		{
			name:       "severity alias",
			content:    `{"severity":"ERROR","msg":"fail"}`,
			wantLevel:  "error",
			absentKeys: []string{"severity"},
		},
		{
			name:       "levelname alias",
			content:    `{"levelname":"WARNING","msg":"caution"}`,
			wantLevel:  "warning",
			absentKeys: []string{"levelname"},
		},
		{
			name:      "unknown level preserved lowercase",
			content:   `{"level":"custom_thing"}`,
			wantLevel: "custom_thing",
		},
		{
			name:      "no level field present",
			content:   `{"msg":"no level here"}`,
			wantLevel: "",
		},
		{
			name:      "level wins over s alias",
			content:   `{"level":"info","s":"W"}`,
			wantLevel: "info",
		},
		{
			name:      "fatal level",
			content:   `{"level":"CRITICAL","msg":"crash"}`,
			wantLevel: "fatal",
		},
		{
			name:      "debug level",
			content:   `{"level":"DEBUG","msg":"verbose"}`,
			wantLevel: "debug",
		},
		{
			name:      "trace level",
			content:   `{"level":"TRACE","msg":"very verbose"}`,
			wantLevel: "trace",
		},
		{
			name:      "plain text no level added",
			content:   `plain text log`,
			wantLevel: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			line := baseLine
			line.Content = tt.content

			got := FormatJSONL(line)

			var obj map[string]any
			if err := json.Unmarshal(got, &obj); err != nil {
				t.Fatalf("invalid JSON: %v\noutput: %s", err, got)
			}

			level, hasLevel := obj["level"]
			if tt.wantLevel == "" {
				if hasLevel {
					t.Errorf("expected no level key, got %q", level)
				}
			} else {
				if !hasLevel {
					t.Fatalf("expected level=%q, but key missing", tt.wantLevel)
				}
				if level != tt.wantLevel {
					t.Errorf("level = %q, want %q", level, tt.wantLevel)
				}
			}

			for _, key := range tt.absentKeys {
				if _, ok := obj[key]; ok {
					t.Errorf("expected alias key %q to be removed, but still present", key)
				}
			}
		})
	}
}
