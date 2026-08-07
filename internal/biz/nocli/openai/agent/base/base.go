package base

import (
	"ai-rag-demo/internal/biz/nocli/checkpoint"
	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/openai/tool"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/external/mcp"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/skill"
	"context"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// BaseAgent 通用 Agent 基础结构体，提供核心 ReAct 循环、Tool 执行与 Stream Fetcher 能力
type BaseAgent struct {
	name            string
	cfg             *conf.Config
	toolRegistry    *tool.Registry
	skillMgr        *skill.Manager
	mcpMgr          mcp.Manager
	maxIterations   int
	tools           []openai.Tool
	model           string
	checkpointStore checkpoint.ICheckpointStore
}

func (b *BaseAgent) SetCheckpointStore(store checkpoint.ICheckpointStore) {
	b.checkpointStore = store
	if b.toolRegistry != nil {
		b.toolRegistry.SetCheckpointStore(store)
	}
}

func (b *BaseAgent) CheckpointStore() checkpoint.ICheckpointStore {
	return b.checkpointStore
}

func (b *BaseAgent) Name() string {
	if b.name != "" {
		return b.name
	}
	return "main"
}

func (b *BaseAgent) SetName(name string) {
	b.name = name
}

func (b *BaseAgent) SetSkillManager(skillMgr *skill.Manager) {
	b.skillMgr = skillMgr
}

func (b *BaseAgent) SkillManager() *skill.Manager {
	return b.skillMgr
}

func (b *BaseAgent) SetMCPManager(mcpMgr mcp.Manager) {
	b.mcpMgr = mcpMgr
}

func (b *BaseAgent) MCPManager() mcp.Manager {
	return b.mcpMgr
}

func NewBaseAgent(name string, cfg *conf.Config, toolRegistry *tool.Registry) *BaseAgent {
	maxIter := 15
	if cfg != nil && cfg.Source.Nocli != nil && cfg.Source.Nocli.MaxAgentIterations > 0 {
		maxIter = cfg.Source.Nocli.MaxAgentIterations
	}

	var defaultModel string
	if cfg != nil && cfg.Source.OpenAI != nil && cfg.Source.OpenAI.Model != "" {
		defaultModel = cfg.Source.OpenAI.Model
	} else {
		defaultModel = "deepseek-v3.2"
	}

	if toolRegistry == nil {
		toolRegistry = tool.NewEmptyRegistry()
	}

	return &BaseAgent{
		name:          name,
		cfg:           cfg,
		toolRegistry:  toolRegistry,
		maxIterations: maxIter,
		model:         defaultModel,
	}
}

func (b *BaseAgent) RegisterTool(t tool.Tool) {
	if b.toolRegistry == nil {
		b.toolRegistry = tool.NewEmptyRegistry()
	}
	b.toolRegistry.Register(t)
}

func (b *BaseAgent) SetToolRegistry(r *tool.Registry) {
	b.toolRegistry = r
}

func (b *BaseAgent) ToolRegistry() *tool.Registry {
	return b.toolRegistry
}

func (b *BaseAgent) Tools() []openai.Tool {
	if b.toolRegistry == nil {
		return []openai.Tool{}
	}
	return b.toolRegistry.BuildTools()
}

// BuildToolsPrompt 动态生成当前 Agent 已绑定注册的所有 Tool 的名称与描述说明
func (b *BaseAgent) BuildToolsPrompt() string {
	tools := b.Tools()
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(AvailableToolsPromptHeader)
	for _, t := range tools {
		if t.Function != nil {
			sb.WriteString(fmt.Sprintf("- **'%s'**: %s\n", t.Function.Name, t.Function.Description))
		}
	}
	return sb.String()
}

// BuildFullSystemPrompt 统一进行 Agent 系统提示词的基类模版组装
// 自动将子类 Agent 自定义的核心人设 (corePrompt) 与基类感知的 Tool 清单、通用安全禁令及 Skill 目录模版进行有机拼接
func (b *BaseAgent) BuildFullSystemPrompt(corePrompt string, skillMgrs ...*skill.Manager) string {
	var sb strings.Builder

	sb.WriteString(strings.TrimSpace(corePrompt))

	toolsPrompt := b.BuildToolsPrompt()
	if toolsPrompt != "" {
		sb.WriteString("\n\n")
		sb.WriteString(toolsPrompt)
	}

	// 📌 统一追加基类全局通用安全边界与行为禁令
	sb.WriteString(SafetyConstraintsPrompt)

	var mgr *skill.Manager
	if len(skillMgrs) > 0 && skillMgrs[0] != nil {
		mgr = skillMgrs[0]
	} else {
		mgr = b.skillMgr
	}

	if mgr != nil {
		skillPrompt := mgr.BuildLevel1PromptForAgent(b.Name())
		if skillPrompt != "" {
			sb.WriteString("\n")
			sb.WriteString(skillPrompt)
		}
	}

	return sb.String()
}

// BuildRAGPromptFromContext 从 Context 中动态解析 RAG 知识库配置并渲染 Prompt 增强块
func BuildRAGPromptFromContext(ctx context.Context) string {
	enableRAG, hasEnableRAG := ctx.Value(ParentEnableRAGKey).(bool)
	if !hasEnableRAG {
		return ""
	}

	kbName, _ := ctx.Value(ParentKBNameKey).(string)
	kbDesc, _ := ctx.Value(ParentKBDescriptionKey).(string)

	if enableRAG {
		return fmt.Sprintf(RAGEnabledPromptTemplate, kbName, kbDesc)
	}

	return RAGDisabledPromptTemplate
}

// BuildMCPPromptFromContext 从 Context 中动态解析 MCP 配置并渲染 Prompt 增强块
func BuildMCPPromptFromContext(ctx context.Context, mcpMgr mcp.Manager) string {
	enableMCP, hasEnableMCP := ctx.Value(ParentEnableMCPKey).(bool)
	if !hasEnableMCP || !enableMCP || mcpMgr == nil || !mcpMgr.IsEnabled() {
		return ""
	}

	mcpTools, err := mcpMgr.ListAllTools(ctx)
	if err != nil || len(mcpTools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n### 🔌 当前激活的 MCP (Model Context Protocol) 扩展能力系统：\n")
	for _, st := range mcpTools {
		sb.WriteString(fmt.Sprintf("- **服务 `%s` / 工具 `%s`**: %s\n", st.ServerName, st.Tool.Name, st.Tool.Description))
	}
	sb.WriteString("当需要使用上述 MCP 功能时，请调用物理工具 `call_mcp_tool`，并在参数中指定对应的 `server_name`、`tool_name` 以及所需的 JSON `arguments`。\n")
	return sb.String()
}

// EnhanceRuntimeMessages 统一在 Agent.Run 前动态增强消息历史 (自动动态刷最新 Skill 目录 + 自动注入 RAG / MCP 状态 Prompt)
func (b *BaseAgent) EnhanceRuntimeMessages(ctx context.Context, messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}

	newMsgs := make([]openai.ChatCompletionMessage, len(messages))
	copy(newMsgs, messages)

	enableSkill, hasEnableSkill := ctx.Value(ParentEnableSkillKey).(bool)
	// 若 ctx 显式传递了 ParentEnableSkillKey 则以 ctx 为准；未传递时默认视为开启 (true)
	shouldEnableSkill := true
	if hasEnableSkill {
		shouldEnableSkill = enableSkill
	}

	// 1. 动态刷新并重新注入最新的 Level 1 Skill Prompt
	if shouldEnableSkill && b.skillMgr != nil {
		latestSkillPrompt := b.skillMgr.BuildLevel1PromptForAgent(b.Name())
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
	} else if !shouldEnableSkill {
		// 当显式关闭 Skill 时，剔除 System Message 中残留的扩展技能系统说明
		if newMsgs[0].Role == openai.ChatMessageRoleSystem {
			if idx := strings.Index(newMsgs[0].Content, "## 🛠️ 扩展技能系统"); idx != -1 {
				newMsgs[0].Content = strings.TrimSpace(newMsgs[0].Content[:idx])
			}
		}
	}

	// 2. 动态注入上下文中的 RAG 状态 Prompt
	ragPrompt := BuildRAGPromptFromContext(ctx)
	if ragPrompt != "" {
		if newMsgs[0].Role == openai.ChatMessageRoleSystem {
			if !strings.Contains(newMsgs[0].Content, "【当前实时 RAG 知识库配置与状态】") {
				newMsgs[0].Content += ragPrompt
			}
		} else {
			newMsgs = append([]openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: ragPrompt,
				},
			}, newMsgs...)
		}
	}

	// 3. 动态注入上下文中的 MCP 状态 Prompt
	if b.mcpMgr != nil {
		mcpPrompt := BuildMCPPromptFromContext(ctx, b.mcpMgr)
		if mcpPrompt != "" {
			if newMsgs[0].Role == openai.ChatMessageRoleSystem {
				if !strings.Contains(newMsgs[0].Content, "### 🔌 当前激活的 MCP") {
					newMsgs[0].Content += mcpPrompt
				}
			}
		}
	}

	return newMsgs
}

func (b *BaseAgent) Model() string {
	return b.model
}

func (b *BaseAgent) SetModel(model string) {
	b.model = model
}

func (b *BaseAgent) ResolveModel(model string) string {
	if model != "" {
		return model
	}
	if b.model != "" {
		return b.model
	}
	if b.cfg != nil && b.cfg.Source.OpenAI != nil && b.cfg.Source.OpenAI.Model != "" {
		return b.cfg.Source.OpenAI.Model
	}
	return "deepseek-v3.2"
}

func (b *BaseAgent) GetMaxIterationsForAgent(agentName string, defaultMax int) int {
	if b.cfg != nil && b.cfg.Source.Nocli != nil {
		if b.cfg.Source.Nocli.Agents != nil {
			if agentCfg, ok := b.cfg.Source.Nocli.Agents[agentName]; ok && agentCfg != nil && agentCfg.MaxIterations > 0 {
				return agentCfg.MaxIterations
			}
		}
		if b.cfg.Source.Nocli.MaxAgentIterations > 0 {
			return b.cfg.Source.Nocli.MaxAgentIterations
		}
	}
	if defaultMax > 0 {
		return defaultMax
	}
	return 15
}

func (b *BaseAgent) MaxIterations() int {
	return b.GetMaxIterationsForAgent(b.name, 15)
}

func (b *BaseAgent) GetTimeoutForAgent(agentName string) time.Duration {
	if b.cfg != nil && b.cfg.Source.Nocli != nil {
		if b.cfg.Source.Nocli.Agents != nil {
			if agentCfg, ok := b.cfg.Source.Nocli.Agents[agentName]; ok && agentCfg != nil && agentCfg.Timeout.Duration > 0 {
				return agentCfg.Timeout.Duration
			}
		}
		if b.cfg.Source.Nocli.ExecTimeout.Duration > 0 {
			return b.cfg.Source.Nocli.ExecTimeout.Duration
		}
	}
	return 5 * time.Minute
}

func (b *BaseAgent) Timeout() time.Duration {
	return b.GetTimeoutForAgent(b.name)
}

func (b *BaseAgent) handleMaxIterationsReached(
	ctx context.Context,
	sessionID, model string,
	maxIterations int,
	messages []openai.ChatCompletionMessage,
	baseFields []interface{},
	emitter StreamEmitter,
	fetcher MessageFetcher,
) (*LoopResult, error) {
	log.Warnw(ctx, "agent_max_iterations_reached", append(baseFields, "max", maxIterations)...)

	stopMsg := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: fmt.Sprintf(MaxIterationsStopUserPromptTemplate, maxIterations),
	}
	messages = append(messages, stopMsg)

	finalReq := openai.ChatCompletionRequest{
		Model:    model,
		Messages: SanitizeMessages(messages),
		Tools:    nil,
	}

	finalMsg, err := fetcher(ctx, finalReq)
	if err != nil || finalMsg.Content == "" {
		finalMsg = openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: fmt.Sprintf(MaxIterationsStopAssistantPromptTemplate, maxIterations),
		}
	}

	messages = append(messages, finalMsg)

	emitter(&pb.StreamChunk{
		Event:     pb.StreamEventType_SET_DONE,
		SessionId: sessionID,
		Status:    pb.SessionStatus_SS_IDLE,
	})

	return &LoopResult{
		Messages: messages,
		Reply:    finalMsg.Content,
		Status:   pb.SessionStatus_SS_IDLE,
	}, nil
}

func SanitizeMessages(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	sanitized := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		if len(m.ToolCalls) == 0 {
			m.ToolCalls = nil
		}
		sanitized[i] = m
	}
	return sanitized
}

// FilterSubAgentMessagesForLLM 过滤掉为 Web 展示保存的子 Agent 内部多轮细节消息 (如 read_files/list_files)
// 保证 LLM 运行时接收到的历史上下文极简、无 Token 冲爆隐患与角色混淆，且仅包含主 Agent 的委派指令与总结 ToolResult。
func FilterSubAgentMessagesForLLM(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(msgs) == 0 {
		return nil
	}

	filtered := make([]openai.ChatCompletionMessage, 0, len(msgs))
	for _, m := range msgs {
		// 如果消息明确标记为子 Agent 的 Name (如 "file_analyzer", "rag_agent" 等且非 "main")，在发给 LLM 时予以排除
		if m.Name != "" && m.Name != "main" && m.Role != openai.ChatMessageRoleUser {
			continue
		}
		filtered = append(filtered, m)
	}

	return filtered
}

func FindUnexecutedToolCalls(messages []openai.ChatCompletionMessage) (int, []openai.ToolCall) {
	if len(messages) == 0 {
		return -1, nil
	}

	assistantIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == openai.ChatMessageRoleAssistant && len(messages[i].ToolCalls) > 0 {
			assistantIdx = i
			break
		}
	}

	if assistantIdx == -1 {
		return -1, nil
	}

	assistantMsg := messages[assistantIdx]

	executedToolIDs := make(map[string]bool)
	for i := assistantIdx + 1; i < len(messages); i++ {
		if messages[i].Role == openai.ChatMessageRoleTool && messages[i].ToolCallID != "" {
			executedToolIDs[messages[i].ToolCallID] = true
		}
	}

	unexecuted := make([]openai.ToolCall, 0, len(assistantMsg.ToolCalls))
	for _, tc := range assistantMsg.ToolCalls {
		if !executedToolIDs[tc.ID] {
			unexecuted = append(unexecuted, tc)
		}
	}

	return assistantIdx, unexecuted
}

func TruncateText(str string, maxChars int) string {
	runes := []rune(str)
	if len(runes) > maxChars {
		return string(runes[:maxChars]) + "..."
	}
	return str
}
