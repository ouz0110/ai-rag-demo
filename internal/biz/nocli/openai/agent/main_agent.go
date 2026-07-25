package agent

import (
	"ai-rag-demo/internal/pkg/skill"
	"fmt"
	"strings"
)

const AgentMain = "main"

// MainAgent 通用多模态与系统工具/Skill 调度主 Agent
type MainAgent struct{}

func NewMainAgent() *MainAgent {
	return &MainAgent{}
}

func (a *MainAgent) Name() string {
	return AgentMain
}

func (a *MainAgent) Description() string {
	return "通用智能 AI 助手，具备多模态意图理解、终端 Shell 命令调度、网页与资源分析及 Skill 扩展指令执行的全功能能力。"
}

func (a *MainAgent) SystemPrompt(workDir string, skillMgr *skill.Manager) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`你是通用智能 AI 助手。你拥有强大的通用任务分析、多模态意图理解、系统工具调度与 Skill 扩展能力。

## 🎯 核心原则
1. **高效准确**：理解用户意图，解答通用问题，并通过工具和技能解决实际任务。
2. **安全沙箱**：所有文件读取与终端 Shell 命令操作均在安全范围内进行。

## 📁 当前工作目录
%s

## 🛠️ 系统基础工具
1. **terminal**：在终端执行 Shell 命令或运行技能脚本（如网页抓取、环境探索、编译构建等）。只读指令自动放行，修改/高危指令自动触发审批。
2. **list_files**：探索项目目录结构，了解文件分布。
3. **read_files**：读取本地文件内容或加载 Skill 的 SOP 指令文件 (SKILL.md)。
`, workDir))

	if skillMgr != nil {
		skillPrompt := skillMgr.BuildLevel1PromptForAgent(a.Name())
		if skillPrompt != "" {
			sb.WriteString(skillPrompt)
		}
	}

	return sb.String()
}
