package filter

import (
	"testing"
)

func TestNewFilter(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		wantErr  bool
	}{
		{
			name:     "valid patterns",
			patterns: []string{"^debug:", "ERROR", "WARN.*timeout"},
			wantErr:  false,
		},
		{
			name:     "empty patterns",
			patterns: []string{},
			wantErr:  false,
		},
		{
			name:     "invalid regex",
			patterns: []string{"[invalid(regex"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewFilter(tt.patterns)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFilterMatch(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		line     string
		want     bool
	}{
		{
			name:     "simple string match",
			patterns: []string{"ERROR"},
			line:     "ERROR: something went wrong",
			want:     true,
		},
		{
			name:     "no match",
			patterns: []string{"ERROR"},
			line:     "INFO: all good",
			want:     false,
		},
		{
			name:     "regex anchor start",
			patterns: []string{"^DEBUG"},
			line:     "DEBUG: message",
			want:     true,
		},
		{
			name:     "regex anchor start no match",
			patterns: []string{"^DEBUG"},
			line:     "Message DEBUG: something",
			want:     false,
		},
		{
			name:     "multiple patterns match first",
			patterns: []string{"ERROR", "WARN", "INFO"},
			line:     "ERROR: critical",
			want:     true,
		},
		{
			name:     "multiple patterns no match",
			patterns: []string{"ERROR", "^WARN$", "INFO"},
			line:     "WARNING: be careful",
			want:     false,
		},
		{
			name:     "multiple patterns match with regex",
			patterns: []string{"^ERROR", "WARN.*timeout"},
			line:     "WARN: connection timeout",
			want:     true,
		},
		{
			name:     "empty filter no match",
			patterns: []string{},
			line:     "any line",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f, err := NewFilter(tt.patterns)
			if err != nil {
				t.Fatalf("NewFilter() error = %v", err)
			}
			got := f.Match(tt.line)
			if got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}
