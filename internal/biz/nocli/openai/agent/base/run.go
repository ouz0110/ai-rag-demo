package base

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/openai/compressor"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/observability"

	openai "github.com/sashabaranov/go-openai"
)

// Run 核心 Agent 循环执行引擎
func (b *BaseAgent) Run(ctx context.Context, opts *RunOptions) (res *LoopResult, err error) {
	if opts == nil {
		return nil, fmt.Errorf("opts 不能为空")
	}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		agentTimeout := b.GetTimeoutForAgent(b.Name())
		if agentTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, agentTimeout)
			defer cancel()
		}
	}

	sessionID := opts.SessionID
	if sessionID != "" && ctx.Value(ParentSessionIDKey) == nil {
		ctx = context.WithValue(ctx, ParentSessionIDKey, sessionID)
	}
	ctx = observability.WithSessionID(ctx, sessionID)
	ctx = observability.WithAgentName(ctx, b.Name())

	// 🎯 触发 Observability Agent 维度 Hook
	obs := observability.GetObserver(ctx)
	agentCtx, endAgent := obs.OnAgentStart(ctx, &observability.AgentRunInfo{
		AgentName:     b.Name(),
		SessionID:     sessionID,
		Model:         b.Model(),
		MaxIterations: b.MaxIterations(),
		Timeout:       b.Timeout(),
	})
	ctx = agentCtx
	defer func() {
		var reply string
		if res != nil {
			reply = res.Reply
		}
		endAgent(reply, err)
	}()

	messages := b.EnhanceRuntimeMessages(ctx, opts.Messages)
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
	allToolDurations := make(map[string]int64)

	// 🎯 检查恢复中断逻辑：查找末尾尚未产生的 tool 消息
	_, unexecutedToolCalls := FindUnexecutedToolCalls(messages)
	if len(unexecutedToolCalls) > 0 {
		log.Debugw(ctx, "resume_pending_tool_calls_execution", append(baseFields, "unexecuted_tools_count", len(unexecutedToolCalls))...)
		res, err := b.ProcessToolCalls(ctx, sessionID, baseFields, unexecutedToolCalls, approvedTools, rejectedTools, &totalToolCalls, emitter)
		if err != nil {
			return nil, err
		}
		for k, v := range res.ToolDurations {
			allToolDurations[k] = v
		}
		if len(res.ExecutedMsgs) > 0 {
			messages = append(messages, res.ExecutedMsgs...)
		}
		if res.HasInterrupt {
			intAgentName := b.Name()
			if res.PendingToolCall != nil && res.PendingToolCall.AgentName != "" {
				intAgentName = res.PendingToolCall.AgentName
			}
			emitter(&pb.StreamChunk{
				Event:            pb.StreamEventType_SET_INTERRUPT,
				AgentName:        intAgentName,
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
				ToolDurations:    allToolDurations,
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

		// 🎯 校验并触发上下文压缩 (发送给 LLM 的切片必须排除子 Agent 的内部细节消息，防止多重角色认知混淆与乱输出)
		messagesForLLM := FilterSubAgentMessagesForLLM(messages)
		if opts.Compressor != nil {
			syncFetcher := opts.SyncFetcher
			if syncFetcher == nil {
				syncFetcher = fetcher
			}

			summarizer := compressor.SummarizerFunc(func(sumCtx context.Context, toSum []openai.ChatCompletionMessage) (string, error) {
				promptMsgs := make([]openai.ChatCompletionMessage, 0, len(toSum)+2)
				promptMsgs = append(promptMsgs, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleSystem,
					Content: CompressSystemPrompt,
				})
				promptMsgs = append(promptMsgs, toSum...)
				promptMsgs = append(promptMsgs, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: "请结合上述历史对话与已有记忆，严格按照要求提炼并输出最新上下文记忆 Checkpoint 摘要。",
				})
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

			onCompressStart := compressor.OnCompressStartFunc(func(origTokens, toCompressCount int) {
				log.Infow(ctx, "agent_context_compression_started",
					"session_id", sessionID,
					"orig_tokens", origTokens,
					"to_compress_count", toCompressCount,
				)
				// 🎯 确定压缩，在开始后台耗时提炼摘要前，第一时间向客户端发送 SET_CONTEXT_COMPRESSED 开始通知事件
				emitter(&pb.StreamChunk{
					Event:     pb.StreamEventType_SET_CONTEXT_COMPRESSED,
					SessionId: sessionID,
					Status:    pb.SessionStatus_SS_RUNNING,
					Text:      "检测到对话上下文较长，正在提炼历史记忆并执行上下文压缩...",
					CompressInfo: &pb.CompressInfo{
						OriginalTokens:     int32(origTokens),
						CompressedMsgCount: int32(toCompressCount),
						CompressCount:      opts.CompressCount + 1,
						SummaryPreview:     "正在提炼历史记忆摘要...",
						Status:             pb.CompressStatus_CS_COMPRESSING,
					},
				})
			})

			compRes, err := opts.Compressor.Compress(ctx, opts.CompressCount, messages, summarizer, onCompressStart)
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

				// 向客户端发送最终 SET_CONTEXT_COMPRESSED 结果通知事件
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
						Status:             pb.CompressStatus_CS_COMPLETED,
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

		// 🎯 触发 Observability LLM 维度 Hook
		llmCtx, endLLM := obs.OnLLMStart(ctx, &observability.LLMCallInfo{
			AgentName:     b.Name(),
			SessionID:     sessionID,
			Model:         model,
			MessagesCount: len(req.Messages),
			ToolsCount:    len(req.Tools),
			Iteration:     iteration,
		})

		msg, err := fetcher(llmCtx, req)
		if err != nil && len(req.Tools) > 0 && (!errors.Is(err, context.Canceled) && ctx.Err() == nil && !strings.Contains(err.Error(), "context canceled")) {
			log.Warnw(ctx, "llm_tools_unsupported_fallback", append(baseFields, "iteration", iteration, "error", err)...)
			req.Tools = nil
			msg, err = fetcher(llmCtx, req)
		}
		endLLM(&msg, err)
		if err != nil {
			// 🎯 1. 检查是否为 Agent 执行超时 (context.DeadlineExceeded)
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "deadline exceeded") || strings.Contains(strings.ToLower(err.Error()), "timeout") {
				return b.handleTimeoutReached(ctx, sessionID, messages, msg.Content, baseFields, emitter, newCheckpointMsg, allToolDurations)
			}

			// 🎯 2. 检查是否为用户主动取消 (context.Canceled)
			if errors.Is(err, context.Canceled) || ctx.Err() != nil || strings.Contains(err.Error(), "context canceled") {
				log.Infow(ctx, "agent_run_canceled_by_user_or_context", append(baseFields, "session_id", sessionID)...)
				if msg.Content != "" || len(msg.ToolCalls) > 0 {
					messages = append(messages, msg)
				}
				return &LoopResult{
					AgentName:        b.Name(),
					Messages:         messages,
					Reply:            msg.Content,
					Status:           pb.SessionStatus_SS_PAUSED,
					NewCheckpointMsg: newCheckpointMsg,
					ToolDurations:    allToolDurations,
				}, nil
			}

			log.Errorw(ctx, "llm_call_error", append(baseFields, "iteration", iteration, "error", err)...)
			if msg.Content != "" || len(msg.ToolCalls) > 0 {
				messages = append(messages, msg)
				return &LoopResult{
					AgentName:        b.Name(),
					Messages:         messages,
					Reply:            msg.Content,
					Status:           pb.SessionStatus_SS_IDLE,
					NewCheckpointMsg: newCheckpointMsg,
					ToolDurations:    allToolDurations,
				}, nil
			}
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
				ToolDurations:    allToolDurations,
			}, nil
		}

		res, err := b.ProcessToolCalls(ctx, sessionID, baseFields, msg.ToolCalls, approvedTools, rejectedTools, &totalToolCalls, emitter)
		if err != nil {
			return nil, err
		}
		for k, v := range res.ToolDurations {
			allToolDurations[k] = v
		}

		if len(res.ExecutedMsgs) > 0 {
			messages = append(messages, res.ExecutedMsgs...)
		}

		if res.HasInterrupt {
			intAgentName := b.Name()
			if res.PendingToolCall != nil && res.PendingToolCall.AgentName != "" {
				intAgentName = res.PendingToolCall.AgentName
			}
			emitter(&pb.StreamChunk{
				Event:            pb.StreamEventType_SET_INTERRUPT,
				AgentName:        intAgentName,
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
				ToolDurations:    allToolDurations,
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
				ToolDurations:    allToolDurations,
			}, nil
		}
	}
}

func (b *BaseAgent) handleTimeoutReached(
	ctx context.Context,
	sessionID string,
	messages []openai.ChatCompletionMessage,
	partialContent string,
	baseFields []interface{},
	emitter StreamEmitter,
	newCheckpointMsg *openai.ChatCompletionMessage,
	allToolDurations map[string]int64,
) (*LoopResult, error) {
	agentTimeout := b.GetTimeoutForAgent(b.Name())
	log.Warnw(ctx, "agent_run_timeout_friendly_fallback", append(baseFields, "session_id", sessionID, "timeout", agentTimeout.String())...)

	isMain := b.Name() == "main"

	var timeoutNotice string
	if isMain {
		timeoutNotice = fmt.Sprintf("\n\n⏱️ 【系统提示：主 Agent (%s) 执行已达到最长超时限制 (%v)】\n💡 已为您保留截至超时前检索与推导的阶段性结果。如需继续深入，可以直接发送“继续”恢复执行。", b.Name(), agentTimeout)
	} else {
		timeoutNotice = fmt.Sprintf("\n\n⏱️ 【子 Agent (%s) 执行超时提醒 (%v)】\n💡 截至超时前已完成部分分析与推导，阶段性结果已返回给主 Agent 供参考。", b.Name(), agentTimeout)
	}

	reply := strings.TrimSpace(partialContent)
	if reply != "" {
		reply += timeoutNotice
	} else {
		reply = strings.TrimLeft(timeoutNotice, "\n")
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: reply,
		Name:    b.Name(),
	})

	// 🎯 区分父子 Agent 决定 Stream 结束信号：
	// - 主 Agent 超时：推送 SET_TEXT_DELTA 并发送 SET_DONE 结束全流程；
	// - 子 Agent 超时：只包装阶段性 ToolResult 返回给主 Agent，绝不提前发送 SET_DONE，确保主 Agent 能正常继续推导！
	if isMain {
		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_TEXT_DELTA,
			SessionId: sessionID,
			Status:    pb.SessionStatus_SS_IDLE,
			Text:      timeoutNotice,
		})

		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_DONE,
			SessionId: sessionID,
			Status:    pb.SessionStatus_SS_IDLE,
		})
	}

	return &LoopResult{
		AgentName:        b.Name(),
		Messages:         messages,
		Reply:            reply,
		Status:           pb.SessionStatus_SS_IDLE,
		NewCheckpointMsg: newCheckpointMsg,
		ToolDurations:    allToolDurations,
	}, nil
}
