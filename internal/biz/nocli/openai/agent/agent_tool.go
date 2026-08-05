package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/checkpoint"
	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	openaierr "ai-rag-demo/internal/biz/nocli/openai/error"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

type AgentToolArgs struct {
	Query string `json:"query"`
}

type AgentToolOptions = base.AgentToolOptions

// AgentTool 将任意 IAgent 包装为一个标准的 Tool 供 MainAgent 调度
type AgentTool struct {
	targetAgent     base.IAgent
	chatModel       *chatmodel.ChatModel
	opts            AgentToolOptions
	checkpointStore checkpoint.ICheckpointStore
}

func (t *AgentTool) SetCheckpointStore(store checkpoint.ICheckpointStore) {
	t.checkpointStore = store
}

func (t *AgentTool) CheckpointStore() checkpoint.ICheckpointStore {
	if t.checkpointStore != nil {
		return t.checkpointStore
	}
	if t.targetAgent != nil {
		return t.targetAgent.CheckpointStore()
	}
	return nil
}

func NewAgentTool(targetAgent base.IAgent, chatModel *chatmodel.ChatModel, opts AgentToolOptions) *AgentTool {
	tool := &AgentTool{
		targetAgent: targetAgent,
		chatModel:   chatModel,
		opts:        opts,
	}
	if targetAgent != nil {
		tool.checkpointStore = targetAgent.CheckpointStore()
	}
	return tool
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

	opts := t.opts
	if reqOpts, ok := ctx.Value(base.ParentAgentToolOptionsKey).(base.AgentToolOptions); ok {
		opts = reqOpts
	}

	log.Debugw(ctx, "agent_tool_delegating",
		"target_agent", t.targetAgent.Name(),
		"query", args.Query,
		"pass_full_ctx", opts.PassFullContextToSubAgent,
		"return_full_ctx", opts.ReturnFullContextToParent,
		"stream_sub_agent", opts.StreamSubAgentExecution,
	)

	// 1. 构造子 Agent 的输入消息列表 (支持 PassFullContextToSubAgent 开关，并安全清洗父级历史)
	var subMessages []openai.ChatCompletionMessage
	if opts.PassFullContextToSubAgent {
		if parentMsgs, ok := ctx.Value(base.ParentMessagesKey).([]openai.ChatCompletionMessage); ok && len(parentMsgs) > 0 {
			cleanParentMsgs := SanitizeParentMessagesForSubAgent(parentMsgs)
			if len(cleanParentMsgs) > 0 {
				subMessages = append(subMessages, cleanParentMsgs...)
			}
		}
	}

	systemPrompt := t.targetAgent.SystemPrompt(".")
	if kbName, ok := ctx.Value(base.ParentKBNameKey).(string); ok && kbName != "" {
		kbDesc, _ := ctx.Value(base.ParentKBDescriptionKey).(string)
		systemPrompt += fmt.Sprintf("\n\n【当前目标知识库配置与范畴】\n- 知识库名称：%s\n- 知识库描述：%s\n请务必评估用户提问与该知识库范畴的关联性。若关联，请先使用 rag_search 进行检索；若检索无相关内容或问题与知识库范畴无关，请客观明确告知用户。", kbName, kbDesc)
	}

	subMessages = append([]openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
			Name:    t.targetAgent.Name(),
		},
	}, subMessages...)

	subMessages = append(subMessages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: args.Query,
	})

	initialSubMsgsLen := len(subMessages)

	// 继承父 Agent 的真实 SessionID
	sessionID, _ := ctx.Value(base.ParentSessionIDKey).(string)

	// 2. 构造子 Agent 的 StreamEmitter (支持 StreamSubAgentExecution 开关，并打上当前 AgentName 标记)
	var subEmitter base.StreamEmitter
	if opts.StreamSubAgentExecution {
		if parentEmitter, ok := ctx.Value(base.ParentEmitterKey).(base.StreamEmitter); ok && parentEmitter != nil {
			subEmitter = func(chunk *pb.StreamChunk) {
				if chunk != nil {
					chunk.AgentName = t.targetAgent.Name()
					if chunk.ToolInfo != nil && !strings.HasPrefix(chunk.ToolInfo.ToolName, "[") {
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
	subAgentStart := time.Now()

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

	subDurationMS := time.Since(subAgentStart).Milliseconds()
	if subDurationMS <= 0 {
		subDurationMS = 1
	}

	// 🎯 核心修复：把子 Agent 内部产生的所有 ToolDurations (如内部 terminal 的耗时) 合并回父级 ParentToolDurations
	if parentDurations, ok := ctx.Value(base.ParentToolDurationsKey).(map[string]int64); ok && parentDurations != nil {
		if loopRes != nil && loopRes.ToolDurations != nil {
			for k, v := range loopRes.ToolDurations {
				if v <= 0 {
					v = 1
				}
				parentDurations[k] = v
			}
		}
		parentToolCallID, _ := ctx.Value(base.ParentToolCallIDKey).(string)
		if parentToolCallID != "" {
			parentDurations[parentToolCallID] = subDurationMS
		}
	}

	if loopRes != nil && (loopRes.Status == pb.SessionStatus_SS_PAUSED || errors.Is(err, context.Canceled) || ctx.Err() != nil) {
		log.Infow(ctx, "sub_agent_paused_saving_checkpoint", "target_agent", t.targetAgent.Name(), "sub_msgs_count", len(loopRes.Messages))

		parentToolCallID, _ := ctx.Value(base.ParentToolCallIDKey).(string)
		kbTenantID, _ := ctx.Value(base.ParentKBTenantIDKey).(string)
		kbID, _ := ctx.Value(base.ParentKBIDKey).(string)
		enableRAG, _ := ctx.Value(base.ParentEnableRAGKey).(bool)
		enableSkill, _ := ctx.Value(base.ParentEnableSkillKey).(bool)
		enableMCP, _ := ctx.Value(base.ParentEnableMCPKey).(bool)
		enableRerank, _ := ctx.Value(base.ParentEnableRerankKey).(bool)
		approvedTools, _ := ctx.Value(base.ParentApprovedToolsKey).(map[string]bool)
		rejectedTools, _ := ctx.Value(base.ParentRejectedToolsKey).(map[string]string)

		// 🎯 构造并保存子 Agent 专属 Pause Checkpoint 快照 (供恢复时从断点秒级续跑)
		cp := &base.SubAgentCheckpoint{
			SessionID:        sessionID,
			TargetAgentName:  t.targetAgent.Name(),
			ParentToolCallID: parentToolCallID,
			SubMessages:      loopRes.Messages,
			AgentToolOptions: base.AgentToolOptions{
				PassFullContextToSubAgent: opts.PassFullContextToSubAgent,
				ReturnFullContextToParent: opts.ReturnFullContextToParent,
				StreamSubAgentExecution:   opts.StreamSubAgentExecution,
			},
			KBTenantID:    kbTenantID,
			KBID:          kbID,
			EnableRAG:     enableRAG,
			EnableSkill:   enableSkill,
			EnableMCP:     enableMCP,
			EnableRerank:  enableRerank,
			ApprovedTools: approvedTools,
			RejectedTools: rejectedTools,
			CreatedAt:     time.Now().Unix(),
		}
		if store := t.CheckpointStore(); store != nil {
			_ = store.Save(ctx, cp)
		}
	}

	if loopRes != nil && loopRes.Status == pb.SessionStatus_SS_INTERRUPTED {
		log.Infow(ctx, "sub_agent_interrupted_saving_checkpoint", "target_agent", t.targetAgent.Name(), "pending_tools_count", len(loopRes.PendingToolCalls))
		var pendingCall *pb.PendingToolCall
		if len(loopRes.PendingToolCalls) > 0 {
			pendingCall = loopRes.PendingToolCalls[0]
			if pendingCall.AgentName == "" {
				pendingCall.AgentName = t.targetAgent.Name()
			}
			if holder, ok := ctx.Value(base.ParentPendingToolCallKey).(**pb.PendingToolCall); ok && holder != nil {
				*holder = pendingCall
			}
		}

		parentToolCallID, _ := ctx.Value(base.ParentToolCallIDKey).(string)
		kbTenantID, _ := ctx.Value(base.ParentKBTenantIDKey).(string)
		kbID, _ := ctx.Value(base.ParentKBIDKey).(string)
		enableRAG, _ := ctx.Value(base.ParentEnableRAGKey).(bool)
		enableSkill, _ := ctx.Value(base.ParentEnableSkillKey).(bool)
		enableMCP, _ := ctx.Value(base.ParentEnableMCPKey).(bool)
		enableRerank, _ := ctx.Value(base.ParentEnableRerankKey).(bool)
		approvedTools, _ := ctx.Value(base.ParentApprovedToolsKey).(map[string]bool)
		rejectedTools, _ := ctx.Value(base.ParentRejectedToolsKey).(map[string]string)

		// 🎯 构造并保存子 Agent 专属 Checkpoint 快照 (包含全部上游配置与 ParentToolCallID)
		cp := &base.SubAgentCheckpoint{
			SessionID:        sessionID,
			InterruptID:      pendingCall.GetInterruptId(),
			TargetAgentName:  t.targetAgent.Name(),
			ParentToolCallID: parentToolCallID,
			SubMessages:      loopRes.Messages,
			PendingToolCall:  pendingCall,
			AgentToolOptions: base.AgentToolOptions{
				PassFullContextToSubAgent: opts.PassFullContextToSubAgent,
				ReturnFullContextToParent: opts.ReturnFullContextToParent,
				StreamSubAgentExecution:   opts.StreamSubAgentExecution,
			},
			KBTenantID:    kbTenantID,
			KBID:          kbID,
			EnableRAG:     enableRAG,
			EnableSkill:   enableSkill,
			EnableMCP:     enableMCP,
			EnableRerank:  enableRerank,
			ApprovedTools: approvedTools,
			RejectedTools: rejectedTools,
			CreatedAt:     time.Now().Unix(),
		}
		if store := t.CheckpointStore(); store != nil {
			_ = store.Save(ctx, cp)
		}

		return "", openaierr.NewInterruptErr(fmt.Sprintf("子 Agent %s 触发指令授权审批中断", t.targetAgent.Name()))
	}

	log.Debugw(ctx, "agent_tool_completed", "target_agent", t.targetAgent.Name(), "reply_len", len(loopRes.Reply))

	// 4. 支持 ReturnFullContextToParent: 精确切片剥离传入的父级历史与 System Prompt，仅追加子 Agent 本次生成的【增量执行过程】
	if opts.ReturnFullContextToParent && len(loopRes.Messages) >= initialSubMsgsLen {
		newSubMsgs := loopRes.Messages[initialSubMsgsLen-1:] // 从子 Agent 接收到的 User 委派指令处切片
		toReturn := make([]openai.ChatCompletionMessage, 0, len(newSubMsgs))
		for _, m := range newSubMsgs {
			if m.Role == openai.ChatMessageRoleSystem {
				continue // 再次校验剔除系统提示词
			}
			// 🎯 强制设置 Name 为当前子 Agent 的 Name，绝不漏标或误标为 main
			m.Name = t.targetAgent.Name()
			// 🎯 转换子 Agent 内部的 User 委派 Task 消息为 Assistant 提示，避免在 Web 前端误渲染为人类用户提问气泡
			if m.Role == openai.ChatMessageRoleUser {
				m.Role = openai.ChatMessageRoleAssistant
				m.Content = fmt.Sprintf("📋 【委派任务指令】: %s", m.Content)
			}
			toReturn = append(toReturn, m)
		}
		if appender, ok := ctx.Value(base.ParentAppenderKey).(func([]openai.ChatCompletionMessage)); ok && appender != nil {
			appender(toReturn)
		}
	}

	return fmt.Sprintf("【子 Agent (%s) 独立执行总结】:\n%s", t.targetAgent.Name(), loopRes.Reply), nil
}

// SanitizeParentMessagesForSubAgent 过滤并清洗透传给子 Agent 的父级历史消息:
// 1. 移除父级所有的 System 提示词 (避免与子 Agent 自身的专属 System 提示词冲突)
// 2. 剥离末尾未响应完成的 ToolCalls 消息 (如父 Agent 刚发起的 delegate_to_* 调用消息)
// 确保发送给子 Agent LLM 的消息切片 100% 符合 OpenAI API 工具闭包规范，达成闭环。
func SanitizeParentMessagesForSubAgent(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(msgs) == 0 {
		return nil
	}

	clean := make([]openai.ChatCompletionMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == openai.ChatMessageRoleSystem {
			continue
		}
		clean = append(clean, m)
	}

	// 倒查并剥离末尾尚未等到 Tool 结果响应的 Assistant(ToolCalls) 消息
	for len(clean) > 0 {
		lastMsg := clean[len(clean)-1]
		if lastMsg.Role == openai.ChatMessageRoleAssistant && len(lastMsg.ToolCalls) > 0 {
			allResponded := true
			for _, tc := range lastMsg.ToolCalls {
				hasResult := false
				for _, checkMsg := range clean {
					if checkMsg.Role == openai.ChatMessageRoleTool && checkMsg.ToolCallID == tc.ID {
						hasResult = true
						break
					}
				}
				if !hasResult {
					allResponded = false
					break
				}
			}
			if !allResponded {
				clean = clean[:len(clean)-1]
				continue
			}
		}
		break
	}

	return clean
}
