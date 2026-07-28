package agent

import (
	"context"
	"encoding/json"
	"fmt"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

type AgentToolArgs struct {
	Query string `json:"query"`
}

type AgentToolOptions struct {
	PassFullContextToSubAgent  bool // 父 -> 子：是否将父 Agent 的完整历史消息透传给子 Agent（默认 false）
	ReturnFullContextToParent bool // 子 -> 父：是否将子 Agent 内部多轮执行历史追加回父 Agent（默认 false）
	StreamSubAgentExecution   bool // 是否将子 Agent 的中间执行过程流式推送给用户（默认 true）
}

// AgentTool 将任意 IAgent 包装为一个标准的 Tool 供 MainAgent 调度
type AgentTool struct {
	targetAgent base.IAgent
	chatModel   *chatmodel.ChatModel
	opts        AgentToolOptions
}

func NewAgentTool(targetAgent base.IAgent, chatModel *chatmodel.ChatModel, opts AgentToolOptions) *AgentTool {
	return &AgentTool{
		targetAgent: targetAgent,
		chatModel:   chatModel,
		opts:        opts,
	}
}

func (t *AgentTool) RequiresApproval(ctx context.Context, argsJSON string) bool {
	return false // 委派给子 Agent 默认自动放行
}

func (t *AgentTool) Definition() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        fmt.Sprintf("delegate_to_%s", t.targetAgent.Name()),
			Description: fmt.Sprintf("将子任务委派给 %s 专门处理。适用场景与能力说明：%s", t.targetAgent.Name(), t.targetAgent.Description()),
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "委派给该 Agent 执行的具体子任务指令或问题描述",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

func (t *AgentTool) Run(ctx context.Context, argsJSON string) (string, error) {
	var args AgentToolArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 AgentTool 参数失败: %v", err)
	}

	log.Debugw(ctx, "agent_tool_delegating", "target_agent", t.targetAgent.Name(), "query", args.Query)

	// 1. 构造子 Agent 的输入消息列表 (支持 PassFullContextToSubAgent 开关)
	var subMessages []openai.ChatCompletionMessage
	if t.opts.PassFullContextToSubAgent {
		if parentMsgs, ok := ctx.Value(base.ParentMessagesKey).([]openai.ChatCompletionMessage); ok && len(parentMsgs) > 0 {
			subMessages = append(subMessages, parentMsgs...)
		}
	}

	systemPrompt := t.targetAgent.SystemPrompt(".", nil)
	if kbName, ok := ctx.Value(base.ParentKBNameKey).(string); ok && kbName != "" {
		kbDesc, _ := ctx.Value(base.ParentKBDescriptionKey).(string)
		systemPrompt += fmt.Sprintf("\n\n【当前目标知识库配置与范畴】\n- 知识库名称：%s\n- 知识库描述：%s\n请务必评估用户提问与该知识库范畴的关联性。若关联，请先使用 rag_search 进行检索；若检索无相关内容或问题与知识库范畴无关，请客观明确告知用户。", kbName, kbDesc)
	}

	subMessages = append([]openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}, subMessages...)

	subMessages = append(subMessages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: args.Query,
	})

	// 继承父 Agent 的真实 SessionID
	sessionID, _ := ctx.Value(base.ParentSessionIDKey).(string)

	// 2. 构造子 Agent 的 StreamEmitter (支持 StreamSubAgentExecution 开关，并打上当前 AgentName 标记)
	var subEmitter base.StreamEmitter
	if t.opts.StreamSubAgentExecution {
		if parentEmitter, ok := ctx.Value(base.ParentEmitterKey).(base.StreamEmitter); ok && parentEmitter != nil {
			subEmitter = func(chunk *pb.StreamChunk) {
				if chunk != nil {
					chunk.AgentName = t.targetAgent.Name()
					if chunk.ToolInfo != nil {
						chunk.ToolInfo.ToolName = fmt.Sprintf("[%s] %s", t.targetAgent.Name(), chunk.ToolInfo.ToolName)
					}
				}
				parentEmitter(chunk)
			}
		}
	}
	if subEmitter == nil {
		subEmitter = base.NoopStreamEmitter
	}

	// 3. 执行子 Agent 独立 ReAct 循环
	fetcher := t.targetAgent.GetStreamFetcher(sessionID, t.chatModel, subEmitter)
	loopRes, err := t.targetAgent.Run(ctx, &base.RunOptions{
		SessionID: sessionID,
		Messages:  subMessages,
		Emitter:   subEmitter,
		Fetcher:   fetcher,
	})

	if err != nil {
		return "", fmt.Errorf("子 Agent %s 执行失败: %v", t.targetAgent.Name(), err)
	}

	log.Debugw(ctx, "agent_tool_completed", "target_agent", t.targetAgent.Name(), "reply_len", len(loopRes.Reply))

	if t.opts.ReturnFullContextToParent {
		if appender, ok := ctx.Value(base.ParentAppenderKey).(func([]openai.ChatCompletionMessage)); ok && appender != nil {
			appender(loopRes.Messages)
		}
	}

	return fmt.Sprintf("【子 Agent (%s) 独立执行总结】:\n%s", t.targetAgent.Name(), loopRes.Reply), nil
}
