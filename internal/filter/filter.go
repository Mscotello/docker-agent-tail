// Package filter provides exclude and mute regex filtering.
package filter

import (
	"fmt"
	"regexp"
)

// Filter compiles and applies regex patterns to filter log lines.
type Filter struct {
	patterns []*regexp.Regexp
}

// NewFilter compiles regex patterns and returns a Filter.
// Returns an error if any pattern is invalid regex.
func NewFilter(patterns []string) (*Filter, error) {
	if len(patterns) == 0 {
		return &Filter{}, nil
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compiling pattern %q: %w", pattern, err)
		}
		compiled = append(compiled, re)
	}

	return &Filter{patterns: compiled}, nil
}

// Match returns true if the line should be excluded (matches any pattern).
func (f *Filter) Match(line string) bool {
	for _, pattern := range f.patterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}
