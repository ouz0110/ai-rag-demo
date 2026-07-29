package agent

import (
	"fmt"

	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	"ai-rag-demo/internal/biz/nocli/openai/tool"
	list_files "ai-rag-demo/internal/biz/nocli/openai/tool/list_files"
	read_files "ai-rag-demo/internal/biz/nocli/openai/tool/read_files"
	terminal "ai-rag-demo/internal/biz/nocli/openai/tool/terminal"
	"ai-rag-demo/internal/conf"

	"github.com/sashabaranov/go-openai"
)

const FileAnalyzerAgentName = "file_analyzer"

type FileAnalyzerAgent struct {
	*base.BaseAgent
}

func NewFileAnalyzerAgent(cfg *conf.Config, baseTools *tool.Registry) *FileAnalyzerAgent {
	// 📌 在此显式声明该 Agent 所需绑定的物理工具集
	tools := baseTools.Filter(
		list_files.ToolName,
		read_files.ToolName,
		terminal.ToolName,
	)
	b := base.NewBaseAgent(FileAnalyzerAgentName, cfg, tools)
	return &FileAnalyzerAgent{
		BaseAgent: b,
	}
}

func (a *FileAnalyzerAgent) Name() string {
	return FileAnalyzerAgentName
}

func (a *FileAnalyzerAgent) Description() string {
	return "专有文件分析专家。擅长文件搜索、项目目录结构浏览、代码与各种文本文件的深入分析、关键词检索及路径定位。当用户询问任何关于文件查看、代码分析、目录探索或文件查找的问题时，必须委派给此工具。"
}

func (a *FileAnalyzerAgent) MaxIterations() int {
	return a.GetMaxIterationsForAgent(a.Name(), 30)
}

func (a *FileAnalyzerAgent) Tools() []openai.Tool {
	return a.ToolRegistry().BuildTools()
}

func (a *FileAnalyzerAgent) SystemPrompt(workDir string) string {
	corePrompt := fmt.Sprintf(`你是一个专有文件与代码分析 Agent 助手，当前工作目录为：%s。

核心职责与原则：
1. 【文件与代码深度探索】：你擅长遍历项目目录、阅读源代码文件、定位具体函数/结构体定义、解析项目配置文件及静态代码分析。
2. 【严格基于物理事实】：绝不凭空捏造不存在的文件路径、代码实现或函数签名。任何信息必须使用 'list_files'、'read_files' 或 'terminal' 工具真实验证。
3. 【结构化结果回复】：分析完成后，向主控 Main Agent 吐出清晰、结构化的分析结论，包括涉及的具体文件路径、关键代码片段与原理解释。`, workDir)

	return a.BuildFullSystemPrompt(corePrompt)
}
