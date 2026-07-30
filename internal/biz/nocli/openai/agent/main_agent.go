package agent

import (
	"fmt"

	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"ai-rag-demo/internal/biz/nocli/openai/tool"
	list_files "ai-rag-demo/internal/biz/nocli/openai/tool/list_files"
	loadskill "ai-rag-demo/internal/biz/nocli/openai/tool/load_skill"
	mcptool "ai-rag-demo/internal/biz/nocli/openai/tool/mcp_tool"
	read_files "ai-rag-demo/internal/biz/nocli/openai/tool/read_files"
	terminal "ai-rag-demo/internal/biz/nocli/openai/tool/terminal"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/external/mcp"
	"ai-rag-demo/internal/pkg/skill"
)

const MainAgentName = "main"

type MainAgent struct {
	*base.BaseAgent
	skillMgr *skill.Manager
	mcpMgr   mcp.Manager
}

func NewMainAgent(cfg *conf.Config, baseTools *tool.Registry, skillMgr *skill.Manager, mcpMgr mcp.Manager) *MainAgent {
	// 📌 在此显式声明 MainAgent 初始绑定的物理工具集
	tools := baseTools.Filter(
		list_files.ToolName,
		read_files.ToolName,
		terminal.ToolName,
		loadskill.ToolName,
		mcptool.ToolName,
	)
	b := base.NewBaseAgent(MainAgentName, cfg, tools)
	b.SetSkillManager(skillMgr)
	b.SetMCPManager(mcpMgr)
	return &MainAgent{
		BaseAgent: b,
		skillMgr:  skillMgr,
		mcpMgr:    mcpMgr,
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

func (a *MainAgent) SystemPrompt(workDir string) string {
	corePrompt := fmt.Sprintf(`你是一个强大、通用且具备深度推理与任务调度能力的多模态 AI 主助手（Main Agent），当前系统的根工作目录为：%s。

核心行为准则与分发机制：
1. 【通用闲聊与答疑】：对于日常对话、通用知识问答、编程解题思路与逻辑推理等无须检索项目特定文件、代码或专有知识库即可回答的问题，你可以直接回答用户。
2. 【专有 Agent 委派与工具调度】：
   - 仔细查阅下文绑定的【当前已绑定的可调用工具列表 (Available Tools System)】。
   - 当遇到特定领域或专业任务（如代码库分析、知识库文档检索、技能 SOP 装载、终端命令操作等），你**必须（MUST）**根据工具的描述，优先选择最匹配的专有委派工具（如 delegate_to_*）或底层物理工具执行！
3. 【严格基于物理事实】：
   - 绝不凭空捏造不存在的文件路径、代码实现、函数签名或工具名称。
   - 只能使用【当前已绑定的可调用工具列表】中真实存在的工具。
4. 【友好能力兜底】：
   - 若用户的需求超出了你及所有当前可用的工具边界，请极其礼貌、清晰且友好地告知用户：“抱歉，目前系统尚未接入该功能。我目前为您提供通用问答、文件与代码库分析、知识库检索以及本地文件探索等能力。”`, workDir)

	return a.BuildFullSystemPrompt(corePrompt)
}
