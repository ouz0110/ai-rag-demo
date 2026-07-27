package agent

import (
	"fmt"
	"strings"

	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"ai-rag-demo/internal/biz/nocli/openai/tool"
	list_files "ai-rag-demo/internal/biz/nocli/openai/tool/list_files"
	read_files "ai-rag-demo/internal/biz/nocli/openai/tool/read_files"
	terminal "ai-rag-demo/internal/biz/nocli/openai/tool/terminal"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/skill"
)

const MainAgentName = "main"

type MainAgent struct {
	*base.BaseAgent
}

func NewMainAgent(cfg *conf.Config, baseTools *tool.Registry) *MainAgent {
	// 📌 在此显式声明 MainAgent 初始绑定的物理工具集
	tools := baseTools.Filter(
		list_files.ToolName,
		read_files.ToolName,
		terminal.ToolName,
	)
	b := base.NewBaseAgent(MainAgentName, cfg, tools)
	return &MainAgent{
		BaseAgent: b,
	}
}

func (a *MainAgent) RegisterSubAgentTool(targetAgent base.IAgent, chatModel *chatmodel.ChatModel, opts AgentToolOptions) {
	agentTool := NewAgentTool(targetAgent, chatModel, opts)
	a.RegisterTool(agentTool)
}

func (a *MainAgent) Name() string {
	return MainAgentName
}

func (a *MainAgent) Description() string {
	return "通用多模态 ReAct 交互主 Agent，支持日常问答、任务调度、文件探索与专有子 Agent 委派"
}

func (a *MainAgent) MaxIterations() int {
	return a.GetMaxIterationsForAgent(a.Name(), 15)
}

func (a *MainAgent) SystemPrompt(workDir string, skillMgr *skill.Manager) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`你是一个强大、通用且具备深度推理与任务调度能力的多模态 AI 主助手（Main Agent），当前系统的根工作目录为：%s。

核心行为准则与分发机制：
1. 【通用闲聊与答疑】：对于日常对话、通用知识问答、编程解题思路与逻辑推理等无须检索项目特定文件、代码或专有知识库即可回答的问题，你可以直接回答用户。
2. 【专有 Agent 委派 (CRITICAL)】：
   - 【文件与代码分析任务】：凡是用户的提问涉及 **文件/代码库分析、具体文件/函数/结构体查找、项目目录架构解读、代码重构、代码实现定位**，你**必须（MUST）**调用 'delegate_to_file_analyzer' 工具委派给文件分析 Agent 深入探索！
   - 【知识库问答任务】：凡是用户的提问涉及 **产品规范、业务文档、规章制度或专有知识库检索**，你**必须（MUST）**调用 'delegate_to_rag_agent' 工具委派给 RAG 知识库 Agent（委派前请结合上文将模糊或简短问题补全为完整实体描述）。
3. 【基础工具使用】：
   - 只有在执行通用辅助检索、简单文件查看或在许可范围内运行终端指令时，才使用 'list_files'、'read_files' 或 'terminal' 工具。
   - 绝不凭空捏造不存在的文件路径、代码实现或函数签名。
4. 【友好能力兜底】：
   - 若用户的需求超出了你及所有可用工具和专有 Agent 的能力边界（例如请求物理操控外部硬件设备、尝试未授权的高危行为等），请极其礼貌、清晰且友好地告知用户：“抱歉，目前系统尚未接入该功能。我目前为您提供通用问答、文件与代码库分析、知识库检索以及本地文件探索等能力。”
`, workDir))

	if skillMgr != nil {
		skillPrompt := skillMgr.BuildLevel1PromptForAgent(a.Name())
		if skillPrompt != "" {
			sb.WriteString("\n" + skillPrompt)
		}
	}

	return sb.String()
}
