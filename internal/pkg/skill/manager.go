package skill

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Manager struct {
	registry *Registry
}

func NewManager(registry *Registry) *Manager {
	return &Manager{
		registry: registry,
	}
}

func (m *Manager) Registry() *Registry {
	return m.registry
}

// BuildLevel1PromptForAgent 动态组装特定 Agent 匹配的 Level 1 技能元数据说明 Prompt
func (m *Manager) BuildLevel1PromptForAgent(agentName string) string {
	if m == nil || m.registry == nil {
		return ""
	}

	skills := m.registry.ListSkills()
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## 🛠️ 扩展技能指南 (Available Skills System)\n")
	sb.WriteString("你拥有一套专有扩展技能（Skills），当用户的需求匹配某种技能的描述时，必须按以下规则触发：\n\n")

	for _, s := range skills {
		skillFilePath := filepath.Join(s.Path, "SKILL.md")
		sb.WriteString(fmt.Sprintf("- **%s**: %s (SOP文件路径: %s)\n", s.Frontmatter.Name, s.Frontmatter.Description, skillFilePath))
	}

	sb.WriteString("\n### 🎯 技能触发与执行规则 (CRITICAL):\n")
	sb.WriteString("1. **意图匹配**：当用户的请求符合上述某个技能的描述（例如包含 URL 读取、网页分析等）时，你必须优先触发该技能。\n")
	sb.WriteString("2. **装载 SOP 步骤**：在执行具体任务前，你**必须（MUST）**先调用 `read_files` 工具，读取该技能的 `SOP文件路径` (即对应的 `SKILL.md`)。\n")
	sb.WriteString("3. **遵照执行**：读取 `SKILL.md` 获取完整的操作指引后，严格按照其定义的 SOP 步骤（例如使用 `terminal` 工具运行对应脚本）获取数据并回答用户。\n")

	return sb.String()
}
