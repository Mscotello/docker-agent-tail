package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// lnavFormatJSON is the lnav format definition for docker-agent-tail JSONL.
const lnavFormatJSON = `{
  "$schema": "https://lnav.org/schemas/format-v1.schema.json",
  "docker_agent_tail_log": {
    "title": "docker-agent-tail",
    "description": "Structured JSONL logs from docker-agent-tail",
    "url": "https://docker-agent-tail.michaelscotello.com",
    "json": true,
    "timestamp-field": "ts",
    "level-field": "level",
    "body-field": "message",
    "line-format": [
      { "field": "__timestamp__", "timestamp-format": "%Y-%m-%dT%H:%M:%S%z" },
      " ",
      { "field": "container", "min-width": 12, "auto-width": true, "align": "left" },
      " ",
      { "field": "__level__", "min-width": 5 },
      " ",
      { "field": "message" }
    ],
    "level": {
      "fatal": "^(?:fatal|critical|emerg)$",
      "error": "^error$",
      "warning": "^warning$",
      "info": "^info$",
      "debug": "^debug$",
      "trace": "^trace$"
    },
    "sample": [
      {
        "line": "{\"ts\":\"2026-03-04T10:30:01.789Z\",\"container\":\"api\",\"stream\":\"stdout\",\"level\":\"info\",\"message\":\"GET /api/users 200 12ms\"}"
      },
      {
        "line": "{\"ts\":\"2026-03-04T10:30:02.100Z\",\"container\":\"worker\",\"stream\":\"stderr\",\"level\":\"error\",\"message\":\"connection timeout\"}"
      },
      {
        "line": "{\"ts\":\"2026-03-04T10:30:03.000Z\",\"container\":\"db\",\"stream\":\"stdout\",\"message\":\"plain text log without level\"}"
      }
    ]
  }
}
`

// LnavFormatJSON returns the lnav format definition for docker-agent-tail JSONL.
func LnavFormatJSON() string {
	return lnavFormatJSON
}

// LnavFormatInstalled reports whether the lnav format is installed at the default location.
func LnavFormatInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, ".lnav", "formats", "installed", "docker-agent-tail.json"))
	return err == nil
}

// EnsureLnavFormat installs the lnav format if it is not already present.
// Errors are non-fatal — lnav is optional. Returns true if the format was installed.
func EnsureLnavFormat() bool {
	if LnavFormatInstalled() {
		return false
	}
	if err := RunLnavInstallTo(""); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not auto-install lnav format: %v\n", err)
		return false
	}
	return true
}

// RunLnav opens the latest (or specified) session in lnav.
func RunLnav(outputDir string, sessionName string) error {
	// Check if lnav is installed
	lnavPath, err := execLookPath("lnav")
	if err != nil {
		return fmt.Errorf("lnav is not installed\n\nInstall it:\n  macOS:  brew install lnav\n  Linux:  apt install lnav\n\nMore info: https://lnav.org")
	}

	// Determine the log file path
	var logPath string
	if sessionName != "" {
		logPath = filepath.Join(outputDir, sessionName, "combined.jsonl")
	} else {
		logPath = filepath.Join(outputDir, "latest", "combined.jsonl")
	}

	// Check that the file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		if sessionName != "" {
			return fmt.Errorf("session not found: %s", filepath.Join(outputDir, sessionName))
		}
		return fmt.Errorf("no log sessions found in %s\n\nStart tailing first:\n  docker-agent-tail --all", outputDir)
	}

	// Ensure lnav format is installed
	EnsureLnavFormat()

	// Exec lnav (replaces the current process)
	return execSyscall(lnavPath, []string{"lnav", logPath}, os.Environ())
}

// execLookPath finds an executable in PATH. Variable for testability.
var execLookPath = exec.LookPath

// execSyscall replaces the current process with the given command. Variable for testability.
var execSyscall = syscall.Exec

// RunLnavInstall installs the lnav format definition to ~/.lnav/formats/installed/.
func RunLnavInstall() error {
	return RunLnavInstallTo("")
}

// RunLnavInstallTo installs the lnav format definition to the specified directory.
// If dir is empty, it defaults to ~/.lnav/formats/installed/.
func RunLnavInstallTo(dir string) error {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolving home directory: %w", err)
		}
		dir = filepath.Join(home, ".lnav", "formats", "installed")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating lnav formats directory: %w", err)
	}

	dest := filepath.Join(dir, "docker-agent-tail.json")
	if err := os.WriteFile(dest, []byte(lnavFormatJSON), 0644); err != nil {
		return fmt.Errorf("writing lnav format file: %w", err)
	}

	fmt.Printf("Installed lnav format: %s\n", dest)
	fmt.Println("Usage: lnav logs/latest/combined.jsonl")
	return nil
}
