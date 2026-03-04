package cli

import (
	"fmt"
	"os"
	"path/filepath"
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
