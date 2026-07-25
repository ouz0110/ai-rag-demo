package agent

import (
	"fmt"
	"strings"

	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	"ai-rag-demo/internal/pkg/skill"
)

type CodeAnalyzerAgent struct {
	*base.BaseAgent
}

func NewCodeAnalyzerAgent(base *base.BaseAgent) *CodeAnalyzerAgent {
	return &CodeAnalyzerAgent{
		BaseAgent: base,
	}
}

func (a *CodeAnalyzerAgent) Name() string {
	return "code_analyzer"
}

func (a *CodeAnalyzerAgent) Description() string {
	return "专有代码分析 Agent，擅长架构分析、Symbol 级代码搜索与静态分析"
}

func (a *CodeAnalyzerAgent) MaxIterations() int {
	return a.GetMaxIterationsForAgent(a.Name(), 30)
}

func (a *CodeAnalyzerAgent) SystemPrompt(workDir string, skillMgr *skill.Manager) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`你是一个代码分析专家，当前工作目录为：%s。

核心职责：
1. 深入理解代码结构，优先通过符号搜索定位定义与实现。
2. 保持分析精准客观，给出清晰的代码架构与依赖路径说明。
`, workDir))

	if skillMgr != nil {
		skillPrompt := skillMgr.BuildLevel1PromptForAgent(a.Name())
		if skillPrompt != "" {
			sb.WriteString("\n" + skillPrompt)
		}
	}

	return sb.String()
}
