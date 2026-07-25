package base

import (
	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/pkg/log"
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// Run 核心 Agent 循环执行引擎
func (b *BaseAgent) Run(ctx context.Context, opts *RunOptions) (*LoopResult, error) {
	if opts == nil {
		return nil, fmt.Errorf("opts 不能为空")
	}

	sessionID := opts.SessionID
	messages := opts.Messages
	model := b.Model()
	tools := b.Tools()
	approvedTools := opts.ApprovedTools
	rejectedTools := opts.RejectedTools
	emitter := opts.Emitter
	fetcher := opts.Fetcher

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

	// 🎯 检查恢复中断逻辑：查找末尾尚未产生的 tool 消息
	_, unexecutedToolCalls := FindUnexecutedToolCalls(messages)
	if len(unexecutedToolCalls) > 0 {
		log.Debugw(ctx, "resume_pending_tool_calls_execution", append(baseFields, "unexecuted_tools_count", len(unexecutedToolCalls))...)
		res, err := b.ProcessToolCalls(ctx, sessionID, baseFields, unexecutedToolCalls, approvedTools, rejectedTools, &totalToolCalls, emitter)
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

	maxIterations := b.MaxIterations()

	for {
		iteration++
		if iteration > maxIterations {
			return b.handleMaxIterationsReached(ctx, sessionID, model, maxIterations, messages, baseFields, emitter, fetcher)
		}

		log.Debugw(ctx, "llm_call_start", append(baseFields, "iteration", iteration, "messages_count", len(messages), "tools_count", len(tools))...)

		req := openai.ChatCompletionRequest{
			Model:    model,
			Messages: SanitizeMessages(messages),
			Tools:    tools,
		}
		if len(req.Tools) == 0 {
			req.Tools = nil
		}

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

		res, err := b.ProcessToolCalls(ctx, sessionID, baseFields, msg.ToolCalls, approvedTools, rejectedTools, &totalToolCalls, emitter)
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
}
