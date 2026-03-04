// Package session provides session directory and symlink management.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Session represents a logging session with metadata and output directory.
type Session struct {
	Dir        string    // Full path to session directory (e.g., logs/2026-03-04-103001/)
	StartTime  time.Time // Session start time
	Command    string    // Original command line
	Containers []string  // Container names being logged
}

// Metadata is written to metadata.json in the session directory.
type Metadata struct {
	StartTime  time.Time `json:"start_time"`
	Command    string    `json:"command"`
	Containers []string  `json:"containers"`
}

// NewSession creates a new logging session with timestamped directory.
// Also creates/updates logs/latest symlink to point to this session.
func NewSession(outputDir string, command string, containers []string) (*Session, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	// Create timestamped session directory
	now := time.Now()
	sessionName := now.Format("2006-01-02-150405")
	sessionDir := filepath.Join(outputDir, sessionName)

	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("creating session directory: %w", err)
	}

	session := &Session{
		Dir:        sessionDir,
		StartTime:  now,
		Command:    command,
		Containers: containers,
	}

	// Write metadata.json
	meta := Metadata{
		StartTime:  now,
		Command:    command,
		Containers: containers,
	}
	metadataPath := filepath.Join(sessionDir, "metadata.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return nil, fmt.Errorf("writing metadata: %w", err)
	}

	// Update logs/latest symlink (atomic via rename)
	latestLink := filepath.Join(outputDir, "latest")
	tmpLink := latestLink + ".tmp"
	_ = os.Remove(tmpLink)
	if err := os.Symlink(sessionName, tmpLink); err != nil {
		return nil, fmt.Errorf("creating latest symlink: %w", err)
	}
	if err := os.Rename(tmpLink, latestLink); err != nil {
		return nil, fmt.Errorf("updating latest symlink: %w", err)
	}

	return session, nil
}
