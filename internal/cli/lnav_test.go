package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLnavFormatJSON(t *testing.T) {
	t.Parallel()

	raw := LnavFormatJSON()

	// Must be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("LnavFormatJSON() is not valid JSON: %v", err)
	}

	// Must have the top-level format key
	format, ok := parsed["docker_agent_tail_log"].(map[string]any)
	if !ok {
		t.Fatal("missing docker_agent_tail_log key")
	}

	requiredFields := []string{"json", "timestamp-field", "level-field", "body-field", "line-format", "sample"}
	for _, field := range requiredFields {
		if _, ok := format[field]; !ok {
			t.Errorf("missing required field %q", field)
		}
	}

	// json must be true
	if format["json"] != true {
		t.Errorf("json field = %v, want true", format["json"])
	}

	// sample must have at least 2 entries
	samples, ok := format["sample"].([]any)
	if !ok {
		t.Fatal("sample field is not an array")
	}
	if len(samples) < 2 {
		t.Errorf("sample has %d entries, want at least 2", len(samples))
	}
}

func TestRunLnavInstallTo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := RunLnavInstallTo(dir); err != nil {
		t.Fatalf("RunLnavInstallTo() error: %v", err)
	}

	dest := filepath.Join(dir, "docker-agent-tail.json")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading installed file: %v", err)
	}

	// Must be valid JSON matching our format
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("installed file is not valid JSON: %v", err)
	}
	if _, ok := parsed["docker_agent_tail_log"]; !ok {
		t.Error("installed file missing docker_agent_tail_log key")
	}
}

func TestRunLnavInstallIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := RunLnavInstallTo(dir); err != nil {
		t.Fatalf("first RunLnavInstallTo() error: %v", err)
	}
	if err := RunLnavInstallTo(dir); err != nil {
		t.Fatalf("second RunLnavInstallTo() error: %v", err)
	}

	// File should still be valid
	dest := filepath.Join(dir, "docker-agent-tail.json")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading installed file: %v", err)
	}
	if !json.Valid(data) {
		t.Error("installed file is not valid JSON after second install")
	}
}
