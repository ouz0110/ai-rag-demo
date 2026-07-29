package agent

import (
	"context"
	"fmt"
	"strings"

	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"ai-rag-demo/internal/biz/nocli/openai/tool"
	list_files "ai-rag-demo/internal/biz/nocli/openai/tool/list_files"
	loadskill "ai-rag-demo/internal/biz/nocli/openai/tool/load_skill"
	read_files "ai-rag-demo/internal/biz/nocli/openai/tool/read_files"
	terminal "ai-rag-demo/internal/biz/nocli/openai/tool/terminal"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/skill"

	openai "github.com/sashabaranov/go-openai"
)

const MainAgentName = "main"

type MainAgent struct {
	*base.BaseAgent
	skillMgr *skill.Manager
}

func NewMainAgent(cfg *conf.Config, baseTools *tool.Registry, skillMgr *skill.Manager) *MainAgent {
	// 📌 在此显式声明 MainAgent 初始绑定的物理工具集
	tools := baseTools.Filter(
		list_files.ToolName,
		read_files.ToolName,
		terminal.ToolName,
		loadskill.ToolName,
	)
	b := base.NewBaseAgent(MainAgentName, cfg, tools)
	return &MainAgent{
		BaseAgent: b,
		skillMgr:  skillMgr,
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
2. 【专有 Agent 委派 (CRITICAL)】：
   - 【文件与代码分析任务】：凡是用户的提问涉及 **文件/代码库分析、具体文件/函数/结构体查找、项目目录架构解读、代码重构、代码实现定位**，你**必须（MUST）**调用 'delegate_to_file_analyzer' 工具委派给文件分析 Agent 深入探索！
3. 【基础工具与技能使用】：
   - 在执行通用辅助检索、文件探查查看、技能 SOP 装载或运行许可命令时，请合理选择并使用绑定的底层工具。
   - 当遇到特定领域或匹配规则的 Skill 场景时，优先使用 'load_skill' 动态装载最新 SOP 说明。
   - 绝不凭空捏造不存在的文件路径、代码实现或函数签名。
4. 【友好能力兜底】：
   - 若用户的需求超出了你及所有可用工具和专有 Agent 的能力边界（例如请求物理操控外部硬件设备、尝试未授权的高危行为等），请极其礼貌、清晰且友好地告知用户：“抱歉，目前系统尚未接入该功能。我目前为您提供通用问答、文件与代码库分析、知识库检索以及本地文件探索等能力。”`, workDir)

	return a.BuildFullSystemPrompt(corePrompt)
}

// Run 扩展 MainAgent 的 Run 方法，动态将 context 中的 RAG 状态、最新 Skill 状态配置暴露给 LLM
func (a *MainAgent) Run(ctx context.Context, opts *base.RunOptions) (*base.LoopResult, error) {
	if opts != nil && len(opts.Messages) > 0 {
		newMsgs := make([]openai.ChatCompletionMessage, len(opts.Messages))
		copy(newMsgs, opts.Messages)

		// 📌 1. 动态刷新并重新注入最新的 Level 1 Skill Prompt（解决系统 Skill 修改后历史会话包含旧 Prompt 的问题）
		if a.skillMgr != nil {
			latestSkillPrompt := a.skillMgr.BuildLevel1PromptForAgent(a.Name())
			if latestSkillPrompt != "" {
				if newMsgs[0].Role == openai.ChatMessageRoleSystem {
					if idx := strings.Index(newMsgs[0].Content, "## 🛠️ 扩展技能系统"); idx != -1 {
						baseContent := strings.TrimSpace(newMsgs[0].Content[:idx])
						newMsgs[0].Content = baseContent + "\n" + latestSkillPrompt
					} else {
						newMsgs[0].Content = strings.TrimSpace(newMsgs[0].Content) + "\n" + latestSkillPrompt
					}
				}
			}
		}

		// 📌 2. 动态注入 RAG 配置与上下文状态
		enableRAG, hasEnableRAG := ctx.Value(base.ParentEnableRAGKey).(bool)
		kbName, _ := ctx.Value(base.ParentKBNameKey).(string)
		kbDesc, _ := ctx.Value(base.ParentKBDescriptionKey).(string)

		var ragPrompt string
		if hasEnableRAG && enableRAG {
			ragPrompt = fmt.Sprintf(`

【当前实时 RAG 知识库配置与状态】
- RAG 检索功能：【已显式开启】
- 关联知识库名称：%s
- 知识库范畴与描述：%s
- 调度准则：用户当前已显式开启 RAG 检索功能。若用户的提问与该知识库范畴相关，或用户意图需要查阅业务文档与专有知识库，请务必调用 'delegate_to_rag_agent' 工具委派给 RAG 知识库 Agent 检索！`, kbName, kbDesc)
		} else if hasEnableRAG && !enableRAG {
			ragPrompt = `

【当前实时 RAG 知识库配置与状态】
- RAG 检索功能：【未开启】
- 调度准则：用户未开启 RAG 检索功能，普通问题直接回答，无需调用 'delegate_to_rag_agent'。`
		}

		if ragPrompt != "" {
			if newMsgs[0].Role == openai.ChatMessageRoleSystem {
				newMsgs[0].Content += ragPrompt
			} else {
				newMsgs = append([]openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: ragPrompt,
					},
				}, newMsgs...)
			}
		}

		optsCopy := *opts
		optsCopy.Messages = newMsgs
		return a.BaseAgent.Run(ctx, &optsCopy)
	}

	return a.BaseAgent.Run(ctx, opts)
}
