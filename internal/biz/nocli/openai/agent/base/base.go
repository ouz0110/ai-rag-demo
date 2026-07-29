package base

import (
	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/openai/tool"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/skill"
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// BaseAgent 通用 Agent 基础结构体，提供核心 ReAct 循环、Tool 执行与 Stream Fetcher 能力
type BaseAgent struct {
	name          string
	cfg           *conf.Config
	toolRegistry  *tool.Registry
	skillMgr      *skill.Manager
	maxIterations int
	tools         []openai.Tool
	model         string
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
	sb.WriteString("\n## 🛠️ 当前已绑定的可调用工具列表 (Available Tools System)\n")
	sb.WriteString("你具备调用以下工具的能力，请根据具体需求选择合适的工具：\n")
	for _, t := range tools {
		if t.Function != nil {
			sb.WriteString(fmt.Sprintf("- **'%s'**: %s\n", t.Function.Name, t.Function.Description))
		}
	}
	return sb.String()
}

// BuildFullSystemPrompt 统一进行 Agent 系统提示词的基类模版组装
// 自动将子类 Agent 自定义的核心人设 (corePrompt) 与基类感知的 Tool 清单及 Skill 目录模版进行有机拼接
func (b *BaseAgent) BuildFullSystemPrompt(corePrompt string, skillMgrs ...*skill.Manager) string {
	var sb strings.Builder

	sb.WriteString(strings.TrimSpace(corePrompt))

	toolsPrompt := b.BuildToolsPrompt()
	if toolsPrompt != "" {
		sb.WriteString("\n\n" + toolsPrompt)
	}

	var mgr *skill.Manager
	if len(skillMgrs) > 0 && skillMgrs[0] != nil {
		mgr = skillMgrs[0]
	} else {
		mgr = b.skillMgr
	}

	if mgr != nil {
		skillPrompt := mgr.BuildLevel1PromptForAgent(b.Name())
		if skillPrompt != "" {
			sb.WriteString("\n" + skillPrompt)
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
		return fmt.Sprintf(`

【当前实时 RAG 知识库配置与状态】
- RAG 检索功能：【已显式开启】
- 关联知识库名称：%s
- 知识库范畴与描述：%s
- 调度准则：用户当前已显式开启 RAG 检索功能。若用户的提问与该知识库范畴相关，或用户意图需要查阅业务文档与专有知识库，请务必调用 'delegate_to_rag_agent' 工具委派给 RAG 知识库 Agent 检索！`, kbName, kbDesc)
	}

	return `

【当前实时 RAG 知识库配置与状态】
- RAG 检索功能：【未开启】
- 调度准则：用户未开启 RAG 检索功能，普通问题直接回答，无需调用 'delegate_to_rag_agent'。`
}

// EnhanceRuntimeMessages 统一在 Agent.Run 前动态增强消息历史 (自动动态刷最新 Skill 目录 + 自动注入 RAG 状态 Prompt)
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
	return b.GetMaxIterationsForAgent("main", 15)
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
		Content: fmt.Sprintf("【系统强制安全指令】Agent 执行轮次已达到设定的最大上限 (%d 轮)。请不要再发起任何工具调用，立刻根据已获取的上下文信息向用户输出总结性的最终回答。", maxIterations),
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
			Content: fmt.Sprintf("任务执行轮次已达到最大上限 (%d 轮)，已强制终止。以上为当前收集到的信息总结。", maxIterations),
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
