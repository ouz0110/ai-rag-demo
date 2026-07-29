package base

import (
	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/openai/compressor"
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
	rawEmitter := opts.Emitter
	if rawEmitter == nil {
		rawEmitter = NoopStreamEmitter
	}

	emitter := func(chunk *pb.StreamChunk) {
		if chunk != nil && chunk.AgentName == "" {
			chunk.AgentName = b.Name()
		}
		rawEmitter(chunk)
	}
	fetcher := opts.Fetcher

	baseFields := []interface{}{
		"session_id", sessionID,
		"agent_name", b.Name(),
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
				AgentName:        b.Name(),
				Messages:         messages,
				Reply:            "包含需要授权确认的操作，请审批后恢复执行",
				Status:           pb.SessionStatus_SS_INTERRUPTED,
				PendingToolCalls: []*pb.PendingToolCall{res.PendingToolCall},
			}, nil
		}
		if res.HasReject {
			log.Infow(ctx, "agent_loop_stopped_by_rejection", append(baseFields, "session_id", sessionID)...)
			emitter(&pb.StreamChunk{
				Event:     pb.StreamEventType_SET_TEXT_DELTA,
				SessionId: sessionID,
				Status:    pb.SessionStatus_SS_IDLE,
				Text:      "\n\n[已根据您的授权指示，终止后续工具调用与推导流程]",
			})
			emitter(&pb.StreamChunk{
				Event:     pb.StreamEventType_SET_DONE,
				SessionId: sessionID,
				Status:    pb.SessionStatus_SS_IDLE,
			})
			return &LoopResult{
				AgentName: b.Name(),
				Messages:  messages,
				Reply:     "操作已被用户拒绝，终止后续流程",
				Status:    pb.SessionStatus_SS_IDLE,
			}, nil
		}
	}

	maxIterations := b.MaxIterations()
	var newCheckpointMsg *openai.ChatCompletionMessage

	for {
		iteration++
		if iteration > maxIterations {
			loopRes, err := b.handleMaxIterationsReached(ctx, sessionID, model, maxIterations, messages, baseFields, emitter, fetcher)
			if loopRes != nil {
				loopRes.NewCheckpointMsg = newCheckpointMsg
			}
			return loopRes, err
		}

		// 🎯 校验并触发上下文压缩
		messagesForLLM := messages
		if opts.Compressor != nil {
			syncFetcher := opts.SyncFetcher
			if syncFetcher == nil {
				syncFetcher = fetcher
			}

			summarizer := compressor.SummarizerFunc(func(sumCtx context.Context, toSum []openai.ChatCompletionMessage) (string, error) {
				promptMsgs := make([]openai.ChatCompletionMessage, 0, len(toSum)+1)
				promptMsgs = append(promptMsgs, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleSystem,
					Content: "你是一个精炼的 AI 上下文摘要助手。请务必将传入的历史对话详细归纳为一份包含【前文探讨核心主题】、【已达成的共识与决策】和【关键参数/输出结果】的上下文记忆摘要。这份摘要将作为 AI 下一轮对话的完整前文记忆，确保关键细节不遗漏。控制在 500 字以内。",
				})
				promptMsgs = append(promptMsgs, toSum...)
				sumReq := openai.ChatCompletionRequest{
					Model:    model,
					Messages: SanitizeMessages(promptMsgs),
				}
				// 🎯 强用非流式 API 进行后台摘要提炼
				sumResp, err := syncFetcher(sumCtx, sumReq)
				if err != nil {
					log.Warnw(ctx, "llm_summarizer_fetcher_failed", "error", err)
					return "", err
				}
				return sumResp.Content, nil
			})

			compRes, err := opts.Compressor.Compress(ctx, opts.CompressCount, messages, summarizer)
			if err == nil && compRes != nil && compRes.IsCompressed {
				messagesForLLM = compRes.CompressedMessages
				if compRes.NewCheckpointMsg != nil {
					newCheckpointMsg = compRes.NewCheckpointMsg
				}

				log.Infow(ctx, "agent_context_compressed",
					"session_id", sessionID,
					"orig_tokens", compRes.OriginalTokens,
					"compressed_tokens", compRes.CompressedTokens,
					"is_max_limit", compRes.IsMaxLimitReached,
				)

				// 向客户端发送 SET_CONTEXT_COMPRESSED 通知事件
				emitter(&pb.StreamChunk{
					Event:     pb.StreamEventType_SET_CONTEXT_COMPRESSED,
					SessionId: sessionID,
					Status:    pb.SessionStatus_SS_RUNNING,
					Text:      compRes.SummaryText,
					CompressInfo: &pb.CompressInfo{
						OriginalTokens:     int32(compRes.OriginalTokens),
						CompressedTokens:   int32(compRes.CompressedTokens),
						CompressedMsgCount: int32(compRes.CompressedCount),
						CompressCount:      opts.CompressCount + 1,
						SummaryPreview:     compRes.SummaryText,
						IsMaxLimitReached:  compRes.IsMaxLimitReached,
					},
				})
			}
		}

		log.Debugw(ctx, "llm_call_start", append(baseFields, "iteration", iteration, "messages_count", len(messagesForLLM), "tools_count", len(tools))...)

		req := openai.ChatCompletionRequest{
			Model:    model,
			Messages: SanitizeMessages(messagesForLLM),
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
				AgentName:        b.Name(),
				Messages:         messages,
				Reply:            msg.Content,
				Status:           pb.SessionStatus_SS_IDLE,
				NewCheckpointMsg: newCheckpointMsg,
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
				AgentName:        b.Name(),
				Messages:         messages,
				Reply:            "包含需要授权确认的操作，请审批后恢复执行",
				Status:           pb.SessionStatus_SS_INTERRUPTED,
				PendingToolCalls: []*pb.PendingToolCall{res.PendingToolCall},
				NewCheckpointMsg: newCheckpointMsg,
			}, nil
		}
		if res.HasReject {
			log.Infow(ctx, "agent_loop_stopped_by_rejection", append(baseFields, "session_id", sessionID)...)
			emitter(&pb.StreamChunk{
				Event:     pb.StreamEventType_SET_TEXT_DELTA,
				SessionId: sessionID,
				Status:    pb.SessionStatus_SS_IDLE,
				Text:      "\n\n[已根据您的授权指示，终止后续工具调用与推导流程]",
			})
			emitter(&pb.StreamChunk{
				Event:     pb.StreamEventType_SET_DONE,
				SessionId: sessionID,
				Status:    pb.SessionStatus_SS_IDLE,
			})
			return &LoopResult{
				AgentName:        b.Name(),
				Messages:         messages,
				Reply:            "操作已被用户拒绝，终止后续流程",
				Status:           pb.SessionStatus_SS_IDLE,
				NewCheckpointMsg: newCheckpointMsg,
			}, nil
		}
	}
}
