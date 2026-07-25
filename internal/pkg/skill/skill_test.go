package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseContent(t *testing.T) {
	raw := `---
name: web-fetcher
description: 抓取并提取指定网页 URL 的 HTML 内容。
allowed-tools: terminal(curl:*)
---

# Web Fetcher Skill

### 步骤
1. 使用 terminal 运行 curl -sL <url>
2. 提取文本内容
`

	s, err := ParseContent([]byte(raw))
	if err != nil {
		t.Fatalf("ParseContent 失败: %v", err)
	}

	if s.Frontmatter.Name != "web-fetcher" {
		t.Errorf("期望名称 web-fetcher, 实际: %s", s.Frontmatter.Name)
	}

	if s.Frontmatter.AllowedTools != "terminal(curl:*)" {
		t.Errorf("期望 allowed-tools terminal(curl:*), 实际: %s", s.Frontmatter.AllowedTools)
	}

	if !strings.Contains(s.Body, "# Web Fetcher Skill") {
		t.Errorf("期望包含 Markdown 标题, 实际: %s", s.Body)
	}
}

func TestRegistryAndManager(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	skillContent := `---
name: test-skill
description: 测试技能描述。
---
# Test Skill Body
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	reg := NewRegistry(tmpDir)
	if err := reg.Scan(); err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	s, ok := reg.GetSkill("test-skill")
	if !ok {
		t.Fatalf("未找到技能 test-skill")
	}
	if s.Frontmatter.Name != "test-skill" {
		t.Errorf("名称不匹配: %s", s.Frontmatter.Name)
	}

	mgr := NewManager(reg)
	prompt := mgr.BuildLevel1PromptForAgent("code_analyzer")
	if !strings.Contains(prompt, "test-skill") || !strings.Contains(prompt, "测试技能描述") {
		t.Errorf("BuildLevel1PromptForAgent 组装失败: %s", prompt)
	}
}
