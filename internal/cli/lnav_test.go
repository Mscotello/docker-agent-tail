package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestRunLnav(t *testing.T) {
	// These subtests mutate package-level variables, so they must run sequentially.

	t.Run("not installed", func(t *testing.T) {
		origLookPath := execLookPath
		t.Cleanup(func() { execLookPath = origLookPath })
		execLookPath = func(file string) (string, error) {
			return "", errors.New("not found")
		}

		err := RunLnav(t.TempDir(), "")
		if err == nil {
			t.Fatal("expected error when lnav not installed")
		}
		if got := err.Error(); !strings.Contains(got, "lnav is not installed") {
			t.Errorf("error = %q, want to contain 'lnav is not installed'", got)
		}
	})

	t.Run("no sessions", func(t *testing.T) {
		origLookPath := execLookPath
		t.Cleanup(func() { execLookPath = origLookPath })
		execLookPath = func(file string) (string, error) {
			return "/usr/bin/lnav", nil
		}

		dir := t.TempDir()
		err := RunLnav(dir, "")
		if err == nil {
			t.Fatal("expected error when no sessions exist")
		}
		if got := err.Error(); !strings.Contains(got, "no log sessions found") {
			t.Errorf("error = %q, want to contain 'no log sessions found'", got)
		}
	})

	t.Run("invalid session name", func(t *testing.T) {
		origLookPath := execLookPath
		t.Cleanup(func() { execLookPath = origLookPath })
		execLookPath = func(file string) (string, error) {
			return "/usr/bin/lnav", nil
		}

		dir := t.TempDir()
		invalidNames := []string{
			"../../../etc",
			"not-a-timestamp",
			"2026-03-04",
			"latest",
			"../../passwd",
		}
		for _, name := range invalidNames {
			err := RunLnav(dir, name)
			if err == nil {
				t.Fatalf("expected error for invalid session name %q", name)
			}
			if got := err.Error(); !strings.Contains(got, "invalid session name") {
				t.Errorf("error for %q = %q, want to contain 'invalid session name'", name, got)
			}
		}
	})

	t.Run("session not found", func(t *testing.T) {
		origLookPath := execLookPath
		t.Cleanup(func() { execLookPath = origLookPath })
		execLookPath = func(file string) (string, error) {
			return "/usr/bin/lnav", nil
		}

		dir := t.TempDir()
		err := RunLnav(dir, "2026-01-01-000000")
		if err == nil {
			t.Fatal("expected error for missing session")
		}
		if got := err.Error(); !strings.Contains(got, "session not found") {
			t.Errorf("error = %q, want to contain 'session not found'", got)
		}
	})

	t.Run("exec latest", func(t *testing.T) {
		origLookPath := execLookPath
		origExec := execSyscall
		t.Cleanup(func() {
			execLookPath = origLookPath
			execSyscall = origExec
		})

		execLookPath = func(file string) (string, error) {
			return "/usr/bin/lnav", nil
		}

		var calledPath string
		var calledArgs []string
		execSyscall = func(argv0 string, argv []string, envv []string) error {
			calledPath = argv0
			calledArgs = argv
			return nil
		}

		dir := t.TempDir()
		sessionDir := filepath.Join(dir, "latest")
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sessionDir, "combined.jsonl"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}

		err := RunLnav(dir, "")
		if err != nil {
			t.Fatalf("RunLnav() error: %v", err)
		}
		if calledPath != "/usr/bin/lnav" {
			t.Errorf("exec path = %q, want /usr/bin/lnav", calledPath)
		}
		wantArgs := []string{"lnav", filepath.Join(dir, "latest", "combined.jsonl")}
		if len(calledArgs) != len(wantArgs) || calledArgs[0] != wantArgs[0] || calledArgs[1] != wantArgs[1] {
			t.Errorf("exec args = %v, want %v", calledArgs, wantArgs)
		}
	})

	t.Run("exec specific session", func(t *testing.T) {
		origLookPath := execLookPath
		origExec := execSyscall
		t.Cleanup(func() {
			execLookPath = origLookPath
			execSyscall = origExec
		})

		execLookPath = func(file string) (string, error) {
			return "/usr/bin/lnav", nil
		}

		var calledArgs []string
		execSyscall = func(argv0 string, argv []string, envv []string) error {
			calledArgs = argv
			return nil
		}

		dir := t.TempDir()
		sessionDir := filepath.Join(dir, "2026-03-04-143700")
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sessionDir, "combined.jsonl"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}

		err := RunLnav(dir, "2026-03-04-143700")
		if err != nil {
			t.Fatalf("RunLnav() error: %v", err)
		}
		wantPath := filepath.Join(dir, "2026-03-04-143700", "combined.jsonl")
		if len(calledArgs) < 2 || calledArgs[1] != wantPath {
			t.Errorf("exec args = %v, want path %s", calledArgs, wantPath)
		}
	})
}
