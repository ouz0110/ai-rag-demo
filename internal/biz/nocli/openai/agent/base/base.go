package base

import (
	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/openai/tool"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/log"
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// BaseAgent 通用 Agent 基础结构体，提供核心 ReAct 循环、Tool 执行与 Stream Fetcher 能力
type BaseAgent struct {
	name          string
	cfg           *conf.Config
	toolRegistry  *tool.Registry
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

func NewBaseAgent(cfg *conf.Config, toolRegistry *tool.Registry) *BaseAgent {
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

	return &BaseAgent{
		cfg:           cfg,
		toolRegistry:  toolRegistry,
		maxIterations: maxIter,
		tools:         nil,
		model:         defaultModel,
	}
}

func (b *BaseAgent) ToolRegistry() *tool.Registry {
	return b.toolRegistry
}

func (b *BaseAgent) Tools() []openai.Tool {
	return b.tools
}

func (b *BaseAgent) SetTools(tools []openai.Tool) {
	b.tools = tools
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
