package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	claudeMDMarker = "## Docker Container Logs"

	claudeMDContent = `## Docker Container Logs

**Never use ` + "`docker logs`" + ` directly.** Use ` + "`docker-agent-tail`" + ` to capture structured JSONL logs.
Logs are at ` + "`logs/latest/combined.jsonl`" + `. Query with ` + "`lnav`" + ` or read directly.
Run ` + "`docker-agent-tail --help`" + ` for flags. See ` + "`.claude/skills/docker-logs/SKILL.md`" + ` for full usage.
`

	contextContent = `## Docker Container Logs

This project uses ` + "`docker-agent-tail`" + ` to tail Docker container logs to disk.
Logs are written to ` + "`logs/latest/`" + ` as JSON Lines. Run ` + "`docker-agent-tail --help`" + ` for usage.
When debugging container issues, read ` + "`logs/latest/combined.jsonl`" + `.`

	skillContent = `---
name: docker-logs
description: Query and analyze Docker container logs tailed by docker-agent-tail. Use when debugging container issues or reading logs.
---

# docker-logs

Skill for working with Docker container logs tailed by docker-agent-tail.

## Log locations

- Combined log: ` + "`logs/latest/combined.jsonl`" + `
- Per-container: ` + "`logs/latest/<container-name>.jsonl`" + `
- Metadata: ` + "`logs/latest/metadata.json`" + `

## Querying logs with lnav (recommended)

Use lnav in non-interactive mode (` + "`-n`" + `) with ` + "`-c`" + ` to run SQL queries from scripts or agents:
` + "```bash" + `
# Count log lines per container
lnav -n -c ';SELECT container, count(*) AS cnt FROM log GROUP BY container ORDER BY cnt DESC' logs/latest/combined.jsonl

# Find all errors
lnav -n -c ';SELECT log_time, container, log_body FROM log WHERE level = "error"' logs/latest/combined.jsonl

# Errors in the last 5 minutes
lnav -n -c ';SELECT log_time, container, log_body FROM log WHERE level = "error" AND log_time > datetime("now", "-5 minutes")' logs/latest/combined.jsonl

# Log volume by container and level
lnav -n -c ';SELECT container, level, count(*) FROM log GROUP BY container, level' logs/latest/combined.jsonl

# Filter to a single container
lnav -n -c ':filter-in api' -c ':write-to -' logs/latest/combined.jsonl
` + "```" + `

Interactive mode (for humans): ` + "`docker-agent-tail lnav`" + `

## Fallback — grep/jq

If lnav is not available:
1. Read ` + "`logs/latest/combined.jsonl`" + ` for overview
2. Grep for ` + "`\"stream\":\"stderr\"`" + ` to find errors
3. Use ` + "`jq 'select(.level==\"error\")'`" + ` on JSONL files
4. Check per-container logs for detailed output

## Log format (JSON Lines)

Each line is a JSON object. Plain text messages are wrapped in an envelope:
` + "```json" + `
{"ts":"2026-03-04T10:30:01.789Z","container":"api","stream":"stdout","message":"GET /api/users 200 12ms"}
` + "```" + `

Structured JSON from containers is merged with metadata:
` + "```json" + `
{"ts":"2026-03-04T10:30:02.100Z","container":"worker","stream":"stdout","level":"info","msg":"Job completed","job_id":"send-email-123"}
` + "```" + `

## Common commands

- Tail all containers: ` + "`docker-agent-tail --all`" + `
- Tail specific containers: ` + "`docker-agent-tail --names api,worker`" + `
- Filter noise: ` + "`docker-agent-tail --all --exclude 'healthcheck|ping'`" + `
- View in lnav: ` + "`docker-agent-tail lnav`" + `
`
)

// AgentHelp returns a structured guide for AI coding agents
func AgentHelp() string {
	return `# docker-agent-tail — AI Agent Guide

## Quick start

Start tailing logs in the background:

  docker-agent-tail --all --output logs/ &

Logs are now being written to disk as JSON Lines. Read them anytime:

  logs/latest/combined.jsonl    — all containers, interleaved
  logs/latest/<name>.jsonl      — per-container logs
  logs/latest/metadata.json     — session info (containers, start time)

## Querying logs with lnav (recommended)

lnav is a log viewer with SQL queries, filtering, and color-coded levels.
The lnav format is auto-installed on first run.

Use lnav -n (non-interactive) with -c to query logs from scripts or agents:

  # Count log lines per container
  lnav -n -c ';SELECT container, count(*) AS cnt FROM log GROUP BY container ORDER BY cnt DESC' logs/latest/combined.jsonl

  # Find all errors
  lnav -n -c ';SELECT log_time, container, log_body FROM log WHERE level = "error"' logs/latest/combined.jsonl

  # Errors in the last 5 minutes
  lnav -n -c ';SELECT log_time, container, log_body FROM log WHERE level = "error" AND log_time > datetime("now", "-5 minutes")' logs/latest/combined.jsonl

  # Log volume by container and level
  lnav -n -c ';SELECT container, level, count(*) FROM log GROUP BY container, level' logs/latest/combined.jsonl

  # Filter to a single container and print
  lnav -n -c ':filter-in api' -c ':write-to -' logs/latest/combined.jsonl

Interactive mode (for humans): docker-agent-tail lnav
Install lnav: brew install lnav (macOS) or apt install lnav (Linux)
Manual format reinstall: docker-agent-tail lnav-install

## Fallback — grep/jq

If lnav is not available:

  1. Read logs/latest/combined.jsonl for an overview
  2. Grep for "stream":"stderr" to find errors
  3. jq 'select(.level=="error")' logs/latest/combined.jsonl
  4. Check per-container logs for detailed context
  5. Review logs/latest/metadata.json for container info

## Log format (JSON Lines)

Plain text container output is wrapped in a JSON envelope:

  {"ts":"2026-03-04T10:30:01.789Z","container":"api","stream":"stdout","message":"GET /api/users 200 12ms"}

Structured JSON from containers is merged with metadata:

  {"ts":"2026-03-04T10:30:02.100Z","container":"worker","stream":"stdout","level":"info","msg":"Job completed","job_id":"send-email-123"}

## Common commands

  docker-agent-tail --all                    # tail all containers
  docker-agent-tail --names api,web          # tail specific containers
  docker-agent-tail --all --exclude 'health' # filter out noise
  docker-agent-tail --all --since 5m         # last 5 minutes only
  docker-agent-tail --compose                # auto-discover compose services

## Common exclude patterns

  --exclude 'healthcheck|health.*check'      # health check endpoints
  --exclude 'ping|alive'                     # liveness probes
  --exclude 'GET /favicon'                   # browser noise

## Background usage

  # Start in background
  docker-agent-tail --all --output logs/ &

  # Check if running
  pgrep -f docker-agent-tail

  # Stop it
  pkill -f docker-agent-tail

## Cleanup

  docker-agent-tail clean              # keep 5 most recent sessions (default)
  docker-agent-tail clean --retain 10  # keep 10 most recent
  docker-agent-tail clean --retain 0   # delete all sessions

## Setup for your project

  docker-agent-tail init    # creates agent config files for Claude/Cursor/Windsurf

## Documentation

  https://docker-agent-tail.michaelscotello.com
  Full docs (agent-readable): https://docker-agent-tail.michaelscotello.com/llms-full.txt
`
}

// RunInit initializes docker-agent-tail config for AI agents
func RunInit(outputDir string) error {
	// Get current working directory - use provided directory or get cwd
	cwd := outputDir
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
	}

	// Detect which agents are present
	cursorDir := filepath.Join(cwd, ".cursor")
	windsurfDir := filepath.Join(cwd, ".windsurf")

	cursorExists := dirExists(cursorDir)
	windsurfExists := dirExists(windsurfDir)

	// Always create .claude/skills/docker-logs/SKILL.md (primary discovery mechanism)
	if err := initClaudeSkill(cwd); err != nil {
		return fmt.Errorf("initializing claude skill: %w", err)
	}
	fmt.Printf("Initialized .claude/skills/docker-logs/SKILL.md\n")

	// Always update CLAUDE.md (lean pointer to skill)
	if err := initClaudeMD(cwd); err != nil {
		return fmt.Errorf("initializing CLAUDE.md: %w", err)
	}
	fmt.Printf("Initialized CLAUDE.md\n")

	// Initialize .cursor/rules/ if .cursor dir exists
	if cursorExists {
		if err := initCursorRules(cwd); err != nil {
			return fmt.Errorf("initializing .cursor/rules: %w", err)
		}
		fmt.Printf("Initialized .cursor/rules/docker-agent-tail.mdc\n")
	}

	// Initialize .windsurf/rules/ if .windsurf dir exists
	if windsurfExists {
		if err := initWindsurfRules(cwd); err != nil {
			return fmt.Errorf("initializing .windsurf/rules: %w", err)
		}
		fmt.Printf("Initialized .windsurf/rules/docker-agent-tail.md\n")
	}

	return nil
}

// initClaudeMD appends the docker-agent-tail section to CLAUDE.md.
// Creates the file if it doesn't exist. Idempotent — skips if marker already present.
func initClaudeMD(cwd string) error {
	claudeMDPath := filepath.Join(cwd, "CLAUDE.md")

	existing, err := os.ReadFile(claudeMDPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading CLAUDE.md: %w", err)
	}

	// Already has our section — nothing to do
	if strings.Contains(string(existing), claudeMDMarker) {
		return nil
	}

	f, err := os.OpenFile(claudeMDPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening CLAUDE.md: %w", err)
	}
	defer f.Close()

	// Add a blank line separator if file has existing content
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("writing separator: %w", err)
		}
	}
	if len(existing) > 0 {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("writing separator: %w", err)
		}
	}

	if _, err := f.WriteString(claudeMDContent); err != nil {
		return fmt.Errorf("writing docker-agent-tail section: %w", err)
	}

	return nil
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// initClaudeSkill creates .claude/skills/docker-logs/SKILL.md with skill content
func initClaudeSkill(cwd string) error {
	skillDir := filepath.Join(cwd, ".claude", "skills", "docker-logs")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("creating .claude/skills/docker-logs directory: %w", err)
	}

	skillFile := filepath.Join(skillDir, "SKILL.md")
	return os.WriteFile(skillFile, []byte(skillContent), 0644)
}

// initCursorRules initializes .cursor/rules/docker-agent-tail.mdc
func initCursorRules(cwd string) error {
	rulesDir := filepath.Join(cwd, ".cursor", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return fmt.Errorf("creating .cursor/rules directory: %w", err)
	}

	ruleFile := filepath.Join(rulesDir, "docker-agent-tail.mdc")
	return os.WriteFile(ruleFile, []byte(contextContent+"\n"), 0644)
}

// initWindsurfRules initializes .windsurf/rules/docker-agent-tail.md
func initWindsurfRules(cwd string) error {
	rulesDir := filepath.Join(cwd, ".windsurf", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return fmt.Errorf("creating .windsurf/rules directory: %w", err)
	}

	ruleFile := filepath.Join(rulesDir, "docker-agent-tail.md")
	return os.WriteFile(ruleFile, []byte(contextContent+"\n"), 0644)
}

