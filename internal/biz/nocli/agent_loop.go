package nocli

import (
	"context"
	"fmt"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

type LoopResult struct {
	Messages         []openai.ChatCompletionMessage
	Reply            string
	Status           pb.SessionStatus
	PendingToolCalls []*pb.PendingToolCall
}

// MessageFetcher 获取 LLM Assistant 消息的策略闭包 (非流式直接获取，流式通过 Recv 推送并组合)
type MessageFetcher func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionMessage, error)

// StreamEmitter 事件推送闭包 (流式模式下推送 SSE Chunk 帧，非流式模式下空操作)
type StreamEmitter func(chunk *pb.StreamChunk)

// NoopStreamEmitter 空操作闭包 (供非流式模式默认使用)
var NoopStreamEmitter StreamEmitter = func(chunk *pb.StreamChunk) {}

// runAgentLoop 通用 Agent 核心循环引擎（模板方法 + 双闭包策略）
func (s *ChatBiz) runAgentLoop(
	ctx context.Context,
	sessionID string,
	messages []openai.ChatCompletionMessage,
	tools []openai.Tool,
	model string,
	approvedTools map[string]bool,
	rejectedTools map[string]string,
	emitter StreamEmitter,
	fetcher MessageFetcher,
) (*LoopResult, error) {
	if model == "" {
		model = s.resolveModel(model)
	}
	if approvedTools == nil {
		approvedTools = make(map[string]bool)
	}
	if rejectedTools == nil {
		rejectedTools = make(map[string]string)
	}
	if emitter == nil {
		emitter = NoopStreamEmitter
	}

	baseFields := []interface{}{
		"session_id", sessionID,
		"model", model,
	}

	log.Debugw(ctx, "agent_loop_start", append(baseFields, "messages_count", len(messages), "tools_count", len(tools))...)
	iteration := 0
	totalToolCalls := 0

	// 🎯 检查是否是从中断恢复的会话：精确定位尚未执行/尚未产生 Tool 响应消息的 pending ToolCalls 集合
	_, unexecutedToolCalls := findUnexecutedToolCalls(messages)
	if len(unexecutedToolCalls) > 0 {
		log.Debugw(ctx, "resume_pending_tool_calls_execution", append(baseFields, "unexecuted_tools_count", len(unexecutedToolCalls))...)
		res, err := s.processToolCalls(ctx, sessionID, baseFields, unexecutedToolCalls, approvedTools, rejectedTools, &totalToolCalls, emitter)
		if err != nil {
			return nil, err
		}
		if len(res.ExecutedMsgs) > 0 {
			messages = append(messages, res.ExecutedMsgs...)
		}
		if res.HasInterrupt {
			emitter(&pb.StreamChunk{
				Event:            pb.StreamEventType_SET_INTERRUPT,
				SessionId:        sessionID,
				Status:           pb.SessionStatus_SS_INTERRUPTED,
				PendingToolCalls: []*pb.PendingToolCall{res.PendingToolCall},
			})
			return &LoopResult{
				Messages:         messages,
				Reply:            "包含需要授权确认的操作，请审批后恢复执行",
				Status:           pb.SessionStatus_SS_INTERRUPTED,
				PendingToolCalls: []*pb.PendingToolCall{res.PendingToolCall},
			}, nil
		}
	}

	for {
		iteration++

		log.Debugw(ctx, "llm_call_start", append(baseFields, "iteration", iteration, "messages_count", len(messages), "tools_count", len(tools))...)

		req := openai.ChatCompletionRequest{
			Model:    model,
			Messages: sanitizeMessages(messages),
			Tools:    tools,
		}
		if len(req.Tools) == 0 {
			req.Tools = nil
		}

		// 🎯 通过策略闭包获取当前轮次组合完成的 Assistant 消息
		msg, err := fetcher(ctx, req)
		if err != nil && len(req.Tools) > 0 {
			log.Warnw(ctx, "llm_tools_unsupported_fallback", append(baseFields, "iteration", iteration, "error", err)...)
			req.Tools = nil
			msg, err = fetcher(ctx, req)
		}
		if err != nil {
			log.Errorw(ctx, "llm_call_error", append(baseFields, "iteration", iteration, "error", err)...)
			return nil, fmt.Errorf("LLM 调用失败: %v", err)
		}

		if msg.Content == "" && len(msg.ToolCalls) == 0 {
			continue
		}

		messages = append(messages, msg)

		// 无 ToolCalls，说明对话正常推导结束
		if len(msg.ToolCalls) == 0 {
			emitter(&pb.StreamChunk{
				Event:     pb.StreamEventType_SET_DONE,
				SessionId: sessionID,
				Status:    pb.SessionStatus_SS_IDLE,
			})
			return &LoopResult{
				Messages: messages,
				Reply:    msg.Content,
				Status:   pb.SessionStatus_SS_IDLE,
			}, nil
		}

		// 🎯 工具统一处理管道 (传入 emitter 闭包)
		res, err := s.processToolCalls(ctx, sessionID, baseFields, msg.ToolCalls, approvedTools, rejectedTools, &totalToolCalls, emitter)
		if err != nil {
			return nil, err
		}

		if len(res.ExecutedMsgs) > 0 {
			messages = append(messages, res.ExecutedMsgs...)
		}

		// 若遭遇高危工具中断拦截，发送中断事件并切出
		if res.HasInterrupt {
			emitter(&pb.StreamChunk{
				Event:            pb.StreamEventType_SET_INTERRUPT,
				SessionId:        sessionID,
				Status:           pb.SessionStatus_SS_INTERRUPTED,
				PendingToolCalls: []*pb.PendingToolCall{res.PendingToolCall},
			})

			return &LoopResult{
				Messages:         messages,
				Reply:            "包含需要授权确认的操作，请审批后恢复执行",
				Status:           pb.SessionStatus_SS_INTERRUPTED,
				PendingToolCalls: []*pb.PendingToolCall{res.PendingToolCall},
			}, nil
		}
	}
}

// sanitizeMessages 净化请求消息中的空 tool_calls 数组，防止 OpenAI 兼容 API 报 400 Request Format Error
func sanitizeMessages(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	sanitized := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		if len(m.ToolCalls) == 0 {
			m.ToolCalls = nil
		}
		sanitized[i] = m
	}
	return sanitized
}

func (s *ChatBiz) resolveModel(model string) string {
	if model != "" {
		return model
	}
	if s.cfg != nil && s.cfg.Source.OpenAI != nil && s.cfg.Source.OpenAI.Model != "" {
		return s.cfg.Source.OpenAI.Model
	}
	return "deepseek-v3.2"
}

// findUnexecutedToolCalls 寻找历史消息末尾尚未执行/未产生 Tool 响应消息的 ToolCalls 集合
func findUnexecutedToolCalls(messages []openai.ChatCompletionMessage) (int, []openai.ToolCall) {
	if len(messages) == 0 {
		return -1, nil
	}

	// 1. 从后往前寻找最后一条带有 ToolCalls 的 Assistant 消息
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

	// 2. 收集该 Assistant 消息之后已经存在的 Tool 响应消息的 ToolCallID 集合
	executedToolIDs := make(map[string]bool)
	for i := assistantIdx + 1; i < len(messages); i++ {
		if messages[i].Role == openai.ChatMessageRoleTool && messages[i].ToolCallID != "" {
			executedToolIDs[messages[i].ToolCallID] = true
		}
	}

	// 3. 精确筛选出尚未产生 Tool 响应消息的 pending ToolCall 列表
	unexecuted := make([]openai.ToolCall, 0, len(assistantMsg.ToolCalls))
	for _, tc := range assistantMsg.ToolCalls {
		if !executedToolIDs[tc.ID] {
			unexecuted = append(unexecuted, tc)
		}
	}

	return assistantIdx, unexecuted
}
