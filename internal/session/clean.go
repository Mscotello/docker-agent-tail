package session

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// sessionDirPattern matches timestamped session directories (e.g., 2026-03-04-103001).
var sessionDirPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-\d{6}$`)

// CleanSessions removes old session directories from outputDir, keeping the
// newest retain count. Returns the list of deleted (or would-be-deleted) directory names.
// Only directories matching the timestamp pattern are considered sessions.
// The "latest" symlink is updated if it pointed to a deleted session.
// If dryRun is true, returns sessions that would be deleted without deleting them.
func CleanSessions(outputDir string, retain int, dryRun ...bool) ([]string, error) {
	isDryRun := len(dryRun) > 0 && dryRun[0]
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading output directory: %w", err)
	}

	// Collect session directories (matching timestamp pattern)
	var sessions []string
	for _, e := range entries {
		if e.IsDir() && sessionDirPattern.MatchString(e.Name()) {
			sessions = append(sessions, e.Name())
		}
	}

	// Sort descending (newest first) — timestamp format sorts chronologically
	sort.Sort(sort.Reverse(sort.StringSlice(sessions)))

	// Nothing to delete
	if len(sessions) <= retain {
		return nil, nil
	}

	// Sessions to delete are everything after the first `retain` entries
	toDelete := sessions[retain:]

	// Dry-run: return what would be deleted without doing it
	if isDryRun {
		return toDelete, nil
	}

	// Read current latest symlink target
	latestLink := filepath.Join(outputDir, "latest")
	latestTarget, _ := os.Readlink(latestLink)

	var deleted []string
	for _, name := range toDelete {
		dir := filepath.Join(outputDir, name)
		if err := os.RemoveAll(dir); err != nil {
			return deleted, fmt.Errorf("removing session %s: %w", name, err)
		}
		deleted = append(deleted, name)
	}

	// If latest symlink pointed to a deleted session, update it
	if latestTarget != "" {
		latestDeleted := false
		for _, name := range deleted {
			if name == latestTarget {
				latestDeleted = true
				break
			}
		}
		if latestDeleted {
			_ = os.Remove(latestLink)
			// Point to newest remaining session if any are left (atomic via rename)
			if retain > 0 && len(sessions) > len(toDelete) {
				tmpLink := latestLink + ".tmp"
				_ = os.Remove(tmpLink)
				if err := os.Symlink(sessions[0], tmpLink); err == nil {
					_ = os.Rename(tmpLink, latestLink)
				}
			}
		}
	}

	return deleted, nil
}
