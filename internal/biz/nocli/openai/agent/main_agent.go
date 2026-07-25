package agent

import (
	"fmt"
	"strings"

	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	"ai-rag-demo/internal/pkg/skill"
)

type MainAgent struct {
	*base.BaseAgent
}

func NewMainAgent(base *base.BaseAgent) *MainAgent {
	return &MainAgent{
		BaseAgent: base,
	}
}

func (a *MainAgent) Name() string {
	return "main"
}

func (a *MainAgent) Description() string {
	return "通用多模态 ReAct 交互 Agent，支持文件检索与终端交互"
}

func (a *MainAgent) MaxIterations() int {
	return a.GetMaxIterationsForAgent(a.Name(), 15)
}

func (a *MainAgent) SystemPrompt(workDir string, skillMgr *skill.Manager) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`你是一个强大且严谨的 Agent 助手，当前工作目录为：%s。

核心行为准则：
1. 【禁止假想】绝不捏造文件路径、代码或函数签名，任何信息必须通过工具验证。
2. 【严格控制】当任务涉及跨步骤操作时，理清步骤依次执行。
3. 【工具使用】按需调用下发的 Tools。如果需要终端执行命令，使用 terminal 工具。
`, workDir))

	if skillMgr != nil {
		skillPrompt := skillMgr.BuildLevel1PromptForAgent(a.Name())
		if skillPrompt != "" {
			sb.WriteString("\n" + skillPrompt)
		}
	}

	return sb.String()
}
