package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	contextContent = `## Docker Container Logs

This project uses ` + "`docker-agent-tail`" + ` to tail Docker container logs to disk.
Logs are written to ` + "`logs/latest/`" + ` as JSON Lines. Run ` + "`docker-agent-tail --help`" + ` for usage.
When debugging container issues, read ` + "`logs/latest/combined.jsonl`" + `.`

	skillContent = `# docker-logs

Skill for working with Docker container logs tailed by docker-agent-tail.

## Log locations

- Combined log: ` + "`logs/latest/combined.jsonl`" + `
- Per-container: ` + "`logs/latest/<container-name>.jsonl`" + `
- Metadata: ` + "`logs/latest/metadata.json`" + `

## Log format (JSON Lines)

Each line is a JSON object. Plain text messages are wrapped in an envelope:
` + "```json" + `
{"ts":"2026-03-04T10:30:01.789Z","container":"api","stream":"stdout","message":"GET /api/users 200 12ms"}
{"ts":"2026-03-04T10:30:01.800Z","container":"api","stream":"stderr","message":"WARN: connection pool exhausted"}
` + "```" + `

Structured JSON from containers is merged with metadata:
` + "```json" + `
{"ts":"2026-03-04T10:30:02.100Z","container":"worker","stream":"stdout","level":"info","msg":"Job completed","job_id":"send-email-123"}
` + "```" + `

## Common commands

- Tail all containers: ` + "`docker-agent-tail --all`" + `
- Tail specific containers: ` + "`docker-agent-tail --name api --name worker`" + `
- Filter noise: ` + "`docker-agent-tail --all --exclude 'healthcheck|ping'`" + `
- Last 100 lines: ` + "`docker-agent-tail --all --tail 100`" + `

## Debugging workflow

1. Read ` + "`logs/latest/combined.jsonl`" + ` for overview
2. Grep for ` + "`\"stream\":\"stderr\"`" + ` to find errors
3. Check per-container logs for detailed output
4. Review ` + "`logs/latest/metadata.json`" + ` for container info
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

## Log format (JSON Lines)

Plain text container output is wrapped in a JSON envelope:

  {"ts":"2026-03-04T10:30:01.789Z","container":"api","stream":"stdout","message":"GET /api/users 200 12ms"}
  {"ts":"2026-03-04T10:30:01.800Z","container":"api","stream":"stderr","message":"WARN: connection pool exhausted"}

Structured JSON from containers is merged with metadata:

  {"ts":"2026-03-04T10:30:02.100Z","container":"worker","stream":"stdout","level":"info","msg":"Job completed","job_id":"send-email-123"}

## Useful commands

  docker-agent-tail --all                    # tail all containers
  docker-agent-tail --names api,web          # tail specific containers
  docker-agent-tail --all --exclude 'health' # filter out noise
  docker-agent-tail --all --since 5m         # last 5 minutes only

## Background usage

  # Start in background
  docker-agent-tail --all --output logs/ &

  # Check if running
  pgrep -f docker-agent-tail

  # Stop it
  pkill -f docker-agent-tail

## Debugging workflow

  1. Read logs/latest/combined.jsonl for an overview
  2. Grep for "stream":"stderr" to find errors
  3. Check per-container logs for detailed context
  4. Review logs/latest/metadata.json for container info

## Cleanup

  docker-agent-tail clean              # keep 5 most recent sessions (default)
  docker-agent-tail clean --retain 10  # keep 10 most recent
  docker-agent-tail clean --retain 0   # delete all sessions

## Setup for your project

  docker-agent-tail init    # creates agent config files for Claude/Cursor/Windsurf

## Log viewing with lnav

  docker-agent-tail lnav-install   # one-time setup
  lnav logs/latest/combined.jsonl  # view with filtering, SQL, colors

## Documentation

  https://docker-agent-tail.michaelscotello.com
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
	claudeDir := filepath.Join(cwd, ".claude")
	cursorDir := filepath.Join(cwd, ".cursor")
	windsurfDir := filepath.Join(cwd, ".windsurf")

	claudeExists := dirExists(claudeDir)
	cursorExists := dirExists(cursorDir)
	windsurfExists := dirExists(windsurfDir)

	if !claudeExists && !cursorExists && !windsurfExists {
		return fmt.Errorf("no agent directories found (.claude, .cursor, or .windsurf)")
	}

	// Initialize .mcp.json (always)
	if err := initMCPJSON(cwd); err != nil {
		return fmt.Errorf("initializing .mcp.json: %w", err)
	}
	fmt.Printf("Initialized .mcp.json\n")

	// Initialize .claude/skills/docker-logs.md if .claude dir exists
	if claudeExists {
		if err := initClaudeSkill(cwd); err != nil {
			return fmt.Errorf("initializing claude skill: %w", err)
		}
		fmt.Printf("Initialized .claude/skills/docker-logs.md\n")
	}

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

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// initClaudeSkill creates .claude/skills/docker-logs.md with skill content
func initClaudeSkill(cwd string) error {
	skillsDir := filepath.Join(cwd, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Errorf("creating .claude/skills directory: %w", err)
	}

	skillFile := filepath.Join(skillsDir, "docker-logs.md")
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

// MCPConfig represents the .mcp.json structure
type MCPConfig struct {
	Tools map[string]interface{} `json:"tools"`
}

// initMCPJSON initializes or updates .mcp.json
func initMCPJSON(cwd string) error {
	mcpPath := filepath.Join(cwd, ".mcp.json")

	// Read existing config if it exists
	var config MCPConfig
	if data, err := os.ReadFile(mcpPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			// If file exists but is invalid JSON, start fresh
			config.Tools = make(map[string]interface{})
		}
	} else {
		config.Tools = make(map[string]interface{})
	}

	// Ensure tools map exists
	if config.Tools == nil {
		config.Tools = make(map[string]interface{})
	}

	// Add or update docker-agent-tail entry
	config.Tools["docker-agent-tail"] = map[string]interface{}{
		"description": "Stream Docker container logs to disk with structured output",
		"enabled":     true,
	}

	// Write back to file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling .mcp.json: %w", err)
	}

	return os.WriteFile(mcpPath, append(data, '\n'), 0644)
}
