package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInit(t *testing.T) {
	t.Run("no agent directories", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		err := RunInit(tmpDir)
		if err == nil {
			t.Fatal("expected error for no agent directories")
		}
		if !strings.Contains(err.Error(), "no agent directories found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("with claude directory writes skill file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		os.Mkdir(filepath.Join(tmpDir, ".claude"), 0755)

		err := RunInit(tmpDir)
		if err != nil {
			t.Fatalf("RunInit failed: %v", err)
		}

		// Verify skill file was created
		skillPath := filepath.Join(tmpDir, ".claude", "skills", "docker-logs.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("skill file not created: %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "# docker-logs") {
			t.Fatal("skill file missing title")
		}
		if !strings.Contains(content, "logs/latest/combined.log") {
			t.Fatal("skill file missing log path")
		}
		if !strings.Contains(content, "Debugging workflow") {
			t.Fatal("skill file missing debugging workflow")
		}
	})

	t.Run("claude does not modify CLAUDE.md", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		os.Mkdir(filepath.Join(tmpDir, ".claude"), 0755)

		// Create existing CLAUDE.md
		existingContent := "# My Project\n\nSome existing content\n"
		claudeMdPath := filepath.Join(tmpDir, "CLAUDE.md")
		os.WriteFile(claudeMdPath, []byte(existingContent), 0644)

		if err := RunInit(tmpDir); err != nil {
			t.Fatalf("RunInit failed: %v", err)
		}

		// Verify CLAUDE.md was NOT modified
		data, err := os.ReadFile(claudeMdPath)
		if err != nil {
			t.Fatalf("CLAUDE.md read failed: %v", err)
		}
		if string(data) != existingContent {
			t.Fatal("CLAUDE.md was modified but should not have been")
		}
	})

	t.Run("claude skill file is idempotent", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		os.Mkdir(filepath.Join(tmpDir, ".claude"), 0755)

		// Run twice
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

		os.Mkdir(filepath.Join(tmpDir, ".claude"), 0755)
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
			filepath.Join(tmpDir, ".mcp.json"),
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err != nil {
				t.Fatalf("file not created: %s", p)
			}
		}
	})
}

func TestInitMCPJSON(t *testing.T) {
	t.Run("creates fresh config", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		err := initMCPJSON(tmpDir)
		if err != nil {
			t.Fatalf("initMCPJSON failed: %v", err)
		}

		mcpPath := filepath.Join(tmpDir, ".mcp.json")
		data, _ := os.ReadFile(mcpPath)

		var config MCPConfig
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if config.Tools["docker-agent-tail"] == nil {
			t.Fatal("docker-agent-tail not in tools")
		}
	})

	t.Run("preserves existing tools", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mcpPath := filepath.Join(tmpDir, ".mcp.json")

		existing := MCPConfig{
			Tools: map[string]interface{}{
				"other-tool": map[string]interface{}{
					"enabled": true,
				},
			},
		}
		data, _ := json.Marshal(existing)
		os.WriteFile(mcpPath, data, 0644)

		if err := initMCPJSON(tmpDir); err != nil {
			t.Fatalf("initMCPJSON failed: %v", err)
		}

		newData, _ := os.ReadFile(mcpPath)
		var config MCPConfig
		json.Unmarshal(newData, &config)

		if config.Tools["other-tool"] == nil {
			t.Fatal("existing tool lost")
		}
		if config.Tools["docker-agent-tail"] == nil {
			t.Fatal("docker-agent-tail not added")
		}
	})
}

func TestInitClaudeSkill(t *testing.T) {
	t.Run("creates skills directory and file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		os.Mkdir(filepath.Join(tmpDir, ".claude"), 0755)

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
