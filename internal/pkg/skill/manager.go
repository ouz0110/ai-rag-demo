package skill

import (
	"fmt"
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

func (m *Manager) GetLatestSkill(name string) (*Skill, bool) {
	if m == nil || m.registry == nil {
		return nil, false
	}
	return m.registry.GetLatestSkill(name)
}

func (m *Manager) ListAvailableSkillNames() []string {
	if m == nil || m.registry == nil {
		return nil
	}
	skills := m.registry.ListSkills()
	names := make([]string, 0, len(skills))
	for _, s := range skills {
		names = append(names, s.Frontmatter.Name)
	}
	return names
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
	sb.WriteString("\n\n## 🛠️ 扩展技能系统 (Available Skills System)\n")
	sb.WriteString("你拥有一套专有扩展技能（Skills）。当用户的提问或意图匹配某种技能的描述时，你必须按照以下规则感知、装载与执行：\n\n")

	for _, s := range skills {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n  - 物理根目录: `%s`\n", s.Frontmatter.Name, s.Frontmatter.Description, s.Path))
	}

	sb.WriteString("\n### 🎯 技能触发、装载与自愈重试规范 (CRITICAL):\n")
	sb.WriteString("1. **自动意图识别**：仔细分析用户提问。一旦用户的诉求符合某种技能的描述（如网页提取、代码扫描等），必须优先选择并触发该技能。\n")
	sb.WriteString("2. **按需动态装载 SOP**：在执行具体任务前，你**必须（MUST）**先调用 `load_skill` 工具（传入 `name` 参数为对应的技能名称），实时获取并装载该技能最新的 SOP 操作规范！\n")
	sb.WriteString("3. **绝对路径自动换算与校验**：若 SOP 文档中涉及脚本或相关文件执行（如 `node scripts/fetch.js`），你在构造 `terminal` 工具命令时，**必须（MUST）主动将相对路径换算为该技能物理根目录下的绝对路径**（例：`node \"<物理根目录>/scripts/fetch.js\" \"<参数>\"`），严禁使用未经换算的相对路径！\n")
	sb.WriteString("4. **执行失败自动诊断与自我修复**：\n")
	sb.WriteString("   - 若脚本或命令执行失败（如报错信息、找不到模块、退出码不为0）：**绝对禁止放弃或直接向用户报错**！\n")
	sb.WriteString("   - 你必须立即分析终端返回的 Stderr 与错误日志，诊断根因。\n")
	sb.WriteString("   - 充分利用现有工具（`read_files` 查看脚本代码、`list_files` 检查文件列表、`terminal` 安装依赖或调整参数），修正路径与执行参数后**自动重试**，直到成功或得出明确结论。\n")

	return sb.String()
}

