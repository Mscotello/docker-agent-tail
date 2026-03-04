package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInit(t *testing.T) {
	t.Run("no agent directories creates skill and CLAUDE.md", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		err := RunInit(tmpDir)
		if err != nil {
			t.Fatalf("expected success with no agent dirs, got: %v", err)
		}

		// Skill file should always be created
		skillPath := filepath.Join(tmpDir, ".claude", "skills", "docker-logs.md")
		if _, err := os.Stat(skillPath); err != nil {
			t.Fatal("skill file not created")
		}

		// CLAUDE.md should be created
		if _, err := os.Stat(filepath.Join(tmpDir, "CLAUDE.md")); err != nil {
			t.Fatal("CLAUDE.md not created")
		}

		// .mcp.json should NOT be created
		if _, err := os.Stat(filepath.Join(tmpDir, ".mcp.json")); !os.IsNotExist(err) {
			t.Fatal(".mcp.json should not be created (no MCP server)")
		}
	})

	t.Run("skill file always created even without .claude dir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		err := RunInit(tmpDir)
		if err != nil {
			t.Fatalf("RunInit failed: %v", err)
		}

		skillPath := filepath.Join(tmpDir, ".claude", "skills", "docker-logs.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("skill file not created: %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "# docker-logs") {
			t.Fatal("skill file missing title")
		}
		if !strings.Contains(content, "logs/latest/combined.jsonl") {
			t.Fatal("skill file missing log path")
		}
		if !strings.Contains(content, "Querying logs with lnav") {
			t.Fatal("skill file missing lnav query section")
		}
	})

	t.Run("creates CLAUDE.md with lean pointer to skill", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		if err := RunInit(tmpDir); err != nil {
			t.Fatalf("RunInit failed: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
		if err != nil {
			t.Fatal("CLAUDE.md not created")
		}
		content := string(data)
		if !strings.Contains(content, "## Docker Container Logs") {
			t.Fatal("CLAUDE.md missing docker-agent-tail section")
		}
		if !strings.Contains(content, ".claude/skills/docker-logs.md") {
			t.Fatal("CLAUDE.md missing skill file reference")
		}
	})

	t.Run("appends to existing CLAUDE.md", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		existingContent := "# My Project\n\nSome existing content\n"
		claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
		os.WriteFile(claudeMDPath, []byte(existingContent), 0644)

		if err := RunInit(tmpDir); err != nil {
			t.Fatalf("RunInit failed: %v", err)
		}

		data, err := os.ReadFile(claudeMDPath)
		if err != nil {
			t.Fatalf("CLAUDE.md read failed: %v", err)
		}
		content := string(data)

		// Original content preserved
		if !strings.Contains(content, "# My Project") {
			t.Fatal("existing content was lost")
		}
		// New section appended
		if !strings.Contains(content, "## Docker Container Logs") {
			t.Fatal("docker-agent-tail section not appended")
		}
	})

	t.Run("CLAUDE.md idempotent on repeat runs", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		if err := RunInit(tmpDir); err != nil {
			t.Fatalf("first RunInit failed: %v", err)
		}
		firstData, _ := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))

		if err := RunInit(tmpDir); err != nil {
			t.Fatalf("second RunInit failed: %v", err)
		}
		secondData, _ := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))

		if string(firstData) != string(secondData) {
			t.Fatal("CLAUDE.md changed on second run — not idempotent")
		}
	})

	t.Run("claude skill file is idempotent", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		// Run twice (no .claude dir pre-created — init should create it)
		if err := RunInit(tmpDir); err != nil {
			t.Fatalf("first RunInit failed: %v", err)
		}
		if err := RunInit(tmpDir); err != nil {
			t.Fatalf("second RunInit failed: %v", err)
		}

		// Verify skill file exists and has correct content
		skillPath := filepath.Join(tmpDir, ".claude", "skills", "docker-logs.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("skill file not found: %v", err)
		}
		if !strings.Contains(string(data), "# docker-logs") {
			t.Fatal("skill file missing title after second run")
		}
	})

	t.Run("with cursor directory", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		os.Mkdir(filepath.Join(tmpDir, ".cursor"), 0755)

		err := RunInit(tmpDir)
		if err != nil {
			t.Fatalf("RunInit failed: %v", err)
		}

		cursorRulePath := filepath.Join(tmpDir, ".cursor", "rules", "docker-agent-tail.mdc")
		data, err := os.ReadFile(cursorRulePath)
		if err != nil {
			t.Fatalf("cursor rule file not created: %v", err)
		}
		if !strings.Contains(string(data), "Docker Container Logs") {
			t.Fatal("cursor rule file missing content")
		}
	})

	t.Run("with windsurf directory", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		os.Mkdir(filepath.Join(tmpDir, ".windsurf"), 0755)

		err := RunInit(tmpDir)
		if err != nil {
			t.Fatalf("RunInit failed: %v", err)
		}

		windsurfRulePath := filepath.Join(tmpDir, ".windsurf", "rules", "docker-agent-tail.md")
		data, err := os.ReadFile(windsurfRulePath)
		if err != nil {
			t.Fatalf("windsurf rule file not created: %v", err)
		}
		if !strings.Contains(string(data), "Docker Container Logs") {
			t.Fatal("windsurf rule file missing content")
		}
	})

	t.Run("all agent types together", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		os.Mkdir(filepath.Join(tmpDir, ".cursor"), 0755)
		os.Mkdir(filepath.Join(tmpDir, ".windsurf"), 0755)

		if err := RunInit(tmpDir); err != nil {
			t.Fatalf("RunInit failed: %v", err)
		}

		// Verify all files created
		paths := []string{
			filepath.Join(tmpDir, ".claude", "skills", "docker-logs.md"),
			filepath.Join(tmpDir, ".cursor", "rules", "docker-agent-tail.mdc"),
			filepath.Join(tmpDir, ".windsurf", "rules", "docker-agent-tail.md"),
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err != nil {
				t.Fatalf("file not created: %s", p)
			}
		}
	})
}

func TestInitClaudeMD(t *testing.T) {
	t.Run("creates new file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		err := initClaudeMD(tmpDir)
		if err != nil {
			t.Fatalf("initClaudeMD failed: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
		if err != nil {
			t.Fatal("CLAUDE.md not created")
		}
		content := string(data)
		if !strings.Contains(content, claudeMDMarker) {
			t.Fatal("missing marker")
		}
		if !strings.Contains(content, "Never use `docker logs` directly") {
			t.Fatal("missing key instruction")
		}
		if !strings.Contains(content, ".claude/skills/docker-logs.md") {
			t.Fatal("missing skill file reference")
		}
	})

	t.Run("appends to existing file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		existing := "# Project\n\nExisting rules.\n"
		os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(existing), 0644)

		if err := initClaudeMD(tmpDir); err != nil {
			t.Fatalf("initClaudeMD failed: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
		content := string(data)
		if !strings.HasPrefix(content, "# Project") {
			t.Fatal("existing content not preserved")
		}
		if !strings.Contains(content, claudeMDMarker) {
			t.Fatal("section not appended")
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		initClaudeMD(tmpDir)
		first, _ := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))

		initClaudeMD(tmpDir)
		second, _ := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))

		if string(first) != string(second) {
			t.Fatal("content changed on second call")
		}
	})

	t.Run("skips when marker already present", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		// Simulate a user who already has a custom docker section
		custom := "# My App\n\n## Docker Container Logs\n\nMy custom docker notes.\n"
		os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(custom), 0644)

		if err := initClaudeMD(tmpDir); err != nil {
			t.Fatalf("initClaudeMD failed: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
		if string(data) != custom {
			t.Fatal("modified file that already had marker")
		}
	})
}

func TestInitClaudeSkill(t *testing.T) {
	t.Run("creates skills directory and file from scratch", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		// No .claude dir pre-created — initClaudeSkill should create it
		err := initClaudeSkill(tmpDir)
		if err != nil {
			t.Fatalf("initClaudeSkill failed: %v", err)
		}

		skillFile := filepath.Join(tmpDir, ".claude", "skills", "docker-logs.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			t.Fatalf("skill file not created: %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "# docker-logs") {
			t.Fatal("skill file missing title")
		}
		if !strings.Contains(content, "docker-agent-tail --all") {
			t.Fatal("skill file missing commands")
		}
	})
}

func TestInitCursorRules(t *testing.T) {
	t.Run("creates rules directory and file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		os.Mkdir(filepath.Join(tmpDir, ".cursor"), 0755)

		err := initCursorRules(tmpDir)
		if err != nil {
			t.Fatalf("initCursorRules failed: %v", err)
		}

		ruleFile := filepath.Join(tmpDir, ".cursor", "rules", "docker-agent-tail.mdc")
		data, err := os.ReadFile(ruleFile)
		if err != nil {
			t.Fatalf("rule file not created: %v", err)
		}
		if !strings.Contains(string(data), "Docker Container Logs") {
			t.Fatal("rule file missing content")
		}
	})
}

func TestInitWindsurfRules(t *testing.T) {
	t.Run("creates rules directory and file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		os.Mkdir(filepath.Join(tmpDir, ".windsurf"), 0755)

		err := initWindsurfRules(tmpDir)
		if err != nil {
			t.Fatalf("initWindsurfRules failed: %v", err)
		}

		ruleFile := filepath.Join(tmpDir, ".windsurf", "rules", "docker-agent-tail.md")
		data, err := os.ReadFile(ruleFile)
		if err != nil {
			t.Fatalf("rule file not created: %v", err)
		}
		if !strings.Contains(string(data), "Docker Container Logs") {
			t.Fatal("rule file missing content")
		}
	})
}

func TestAgentHelp(t *testing.T) {
	t.Parallel()

	help := AgentHelp()
	if !strings.Contains(help, "llms-full.txt") {
		t.Fatal("AgentHelp missing llms-full.txt link")
	}
}
