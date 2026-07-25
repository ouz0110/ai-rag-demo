package agent

import (
	"ai-rag-demo/internal/pkg/skill"
	"fmt"
	"strings"
)

const AgentCodeAnalyzer = "code_analyzer"

// CodeAnalyzerAgent 专注于代码架构分析的 Agent
type CodeAnalyzerAgent struct{}

func NewCodeAnalyzerAgent() *CodeAnalyzerAgent {
	return &CodeAnalyzerAgent{}
}

func (a *CodeAnalyzerAgent) Name() string {
	return AgentCodeAnalyzer
}

func (a *CodeAnalyzerAgent) Description() string {
	return "专业代码库分析专家，专注于探讨代码架构、代码质量评估、目录扫描与导出项分析。"
}

func (a *CodeAnalyzerAgent) SystemPrompt(workDir string, skillMgr *skill.Manager) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`你是专业代码库分析专家。你的核心职责是深入分析指定工作目录下的代码架构、业务逻辑与文件导出项。

## 📁 当前工作目录
%s

## 🛠️ 工具策略
1. **list_files**：整体扫描与探索目录结构。
2. **read_files**：按需深度读取关键代码文件。
3. **terminal**：需要构建、运行单元测试或提取环境信息时使用。

## 📊 输出规范
提供结构化的 Markdown 报告，包含目录清单、架构依赖说明与代码优化建议。
`, workDir))

	// if skillMgr != nil {
	// 	skillPrompt := skillMgr.BuildLevel1PromptForAgent(a.Name())
	// 	if skillPrompt != "" {
	// 		sb.WriteString(skillPrompt)
	// 	}
	// }

	return sb.String()
}
