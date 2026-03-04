package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanSessions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		sessions        []string // session dirs to create
		otherEntries    []string // non-session entries (should be ignored)
		latestTarget    string   // symlink target for "latest"
		retain          int
		wantDeleted     []string
		wantRemaining   []string
		wantLatest      string // expected latest symlink target after clean
		wantLatestGone  bool   // expect latest symlink to be removed entirely
	}{
		{
			name:          "no sessions exist, retain 5",
			sessions:      nil,
			retain:        5,
			wantDeleted:   nil,
			wantRemaining: nil,
		},
		{
			name:          "fewer sessions than retain",
			sessions:      []string{"2026-03-01-100000", "2026-03-02-100000", "2026-03-03-100000"},
			retain:        5,
			wantDeleted:   nil,
			wantRemaining: []string{"2026-03-01-100000", "2026-03-02-100000", "2026-03-03-100000"},
		},
		{
			name:          "exact match, no deletion",
			sessions:      []string{"2026-03-01-100000", "2026-03-02-100000", "2026-03-03-100000"},
			retain:        3,
			wantDeleted:   nil,
			wantRemaining: []string{"2026-03-01-100000", "2026-03-02-100000", "2026-03-03-100000"},
		},
		{
			name: "7 sessions, retain 3, delete 4 oldest",
			sessions: []string{
				"2026-03-01-100000",
				"2026-03-02-100000",
				"2026-03-03-100000",
				"2026-03-04-100000",
				"2026-03-05-100000",
				"2026-03-06-100000",
				"2026-03-07-100000",
			},
			latestTarget: "2026-03-07-100000",
			retain:       3,
			wantDeleted: []string{
				"2026-03-04-100000",
				"2026-03-03-100000",
				"2026-03-02-100000",
				"2026-03-01-100000",
			},
			wantRemaining: []string{"2026-03-05-100000", "2026-03-06-100000", "2026-03-07-100000"},
			wantLatest:    "2026-03-07-100000",
		},
		{
			name: "retain 0 deletes all",
			sessions: []string{
				"2026-03-01-100000",
				"2026-03-02-100000",
				"2026-03-03-100000",
			},
			latestTarget: "2026-03-03-100000",
			retain:       0,
			wantDeleted: []string{
				"2026-03-03-100000",
				"2026-03-02-100000",
				"2026-03-01-100000",
			},
			wantRemaining:  nil,
			wantLatestGone: true,
		},
		{
			name: "non-session entries are ignored",
			sessions: []string{
				"2026-03-01-100000",
				"2026-03-02-100000",
				"2026-03-03-100000",
			},
			otherEntries:  []string{"readme.txt", "some-dir", ".hidden"},
			latestTarget:  "2026-03-03-100000",
			retain:        1,
			wantDeleted:   []string{"2026-03-02-100000", "2026-03-01-100000"},
			wantRemaining: []string{"2026-03-03-100000"},
			wantLatest:    "2026-03-03-100000",
		},
		{
			name: "latest symlink updated when target deleted",
			sessions: []string{
				"2026-03-01-100000",
				"2026-03-02-100000",
				"2026-03-03-100000",
			},
			latestTarget:  "2026-03-01-100000",
			retain:        2,
			wantDeleted:   []string{"2026-03-01-100000"},
			wantRemaining: []string{"2026-03-02-100000", "2026-03-03-100000"},
			wantLatest:    "2026-03-03-100000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()

			// Create session directories
			for _, s := range tt.sessions {
				if err := os.MkdirAll(filepath.Join(tmpDir, s), 0755); err != nil {
					t.Fatalf("creating session dir %s: %v", s, err)
				}
			}

			// Create non-session entries
			for _, e := range tt.otherEntries {
				path := filepath.Join(tmpDir, e)
				if err := os.MkdirAll(path, 0755); err != nil {
					t.Fatalf("creating other entry %s: %v", e, err)
				}
			}

			// Create latest symlink
			if tt.latestTarget != "" {
				latestLink := filepath.Join(tmpDir, "latest")
				if err := os.Symlink(tt.latestTarget, latestLink); err != nil {
					t.Fatalf("creating latest symlink: %v", err)
				}
			}

			deleted, err := CleanSessions(tmpDir, tt.retain)
			if err != nil {
				t.Fatalf("CleanSessions() error: %v", err)
			}

			// Check deleted list
			if len(deleted) != len(tt.wantDeleted) {
				t.Errorf("deleted count = %d, want %d", len(deleted), len(tt.wantDeleted))
			}
			for i, d := range deleted {
				if i < len(tt.wantDeleted) && d != tt.wantDeleted[i] {
					t.Errorf("deleted[%d] = %q, want %q", i, d, tt.wantDeleted[i])
				}
			}

			// Check remaining directories
			for _, s := range tt.wantRemaining {
				if _, err := os.Stat(filepath.Join(tmpDir, s)); os.IsNotExist(err) {
					t.Errorf("expected session %s to exist, but it was deleted", s)
				}
			}

			// Check deleted directories are gone
			for _, d := range tt.wantDeleted {
				if _, err := os.Stat(filepath.Join(tmpDir, d)); !os.IsNotExist(err) {
					t.Errorf("expected session %s to be deleted, but it still exists", d)
				}
			}

			// Check non-session entries are still present
			for _, e := range tt.otherEntries {
				if _, err := os.Stat(filepath.Join(tmpDir, e)); os.IsNotExist(err) {
					t.Errorf("non-session entry %s was deleted (should be preserved)", e)
				}
			}

			// Check latest symlink
			latestLink := filepath.Join(tmpDir, "latest")
			if tt.wantLatestGone {
				if _, err := os.Lstat(latestLink); !os.IsNotExist(err) {
					t.Error("expected latest symlink to be removed")
				}
			} else if tt.wantLatest != "" {
				target, err := os.Readlink(latestLink)
				if err != nil {
					t.Fatalf("reading latest symlink: %v", err)
				}
				if target != tt.wantLatest {
					t.Errorf("latest symlink = %q, want %q", target, tt.wantLatest)
				}
			}
		})
	}
}

func TestCleanSessions_dryRun(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	sessions := []string{
		"2026-03-01-100000",
		"2026-03-02-100000",
		"2026-03-03-100000",
		"2026-03-04-100000",
		"2026-03-05-100000",
	}
	for _, s := range sessions {
		if err := os.MkdirAll(filepath.Join(tmpDir, s), 0755); err != nil {
			t.Fatalf("creating session dir: %v", err)
		}
	}

	// Dry-run should return sessions to delete but not actually delete them
	deleted, err := CleanSessions(tmpDir, 2, true)
	if err != nil {
		t.Fatalf("CleanSessions(dry-run) error: %v", err)
	}

	if len(deleted) != 3 {
		t.Errorf("dry-run reported %d deletions, want 3", len(deleted))
	}

	// Verify all sessions still exist
	for _, s := range sessions {
		if _, err := os.Stat(filepath.Join(tmpDir, s)); os.IsNotExist(err) {
			t.Errorf("session %s was deleted during dry-run", s)
		}
	}
}

func TestCleanSessions_nonexistent_directory(t *testing.T) {
	t.Parallel()

	deleted, err := CleanSessions("/nonexistent/path/that/does/not/exist", 5)
	if err != nil {
		t.Fatalf("expected no error for nonexistent dir, got: %v", err)
	}
	if len(deleted) != 0 {
		t.Errorf("expected no deletions, got %d", len(deleted))
	}
}
