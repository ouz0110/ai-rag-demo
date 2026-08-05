package nocli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/openai/agent"
	agentbase "ai-rag-demo/internal/biz/nocli/openai/agent/base"

	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

// StreamCompletion 流式完成接口 (由 Service 层直接传入 StreamEmitter 闭包)
func (s *ChatBiz) StreamCompletion(ctx context.Context, req *pb.CompletionRequest, emitter agentbase.StreamEmitter) error {
	if emitter == nil {
		emitter = agentbase.NoopStreamEmitter
	}

	var sessionID string
	var messages []openai.ChatCompletionMessage
	var newMessageStart int
	var err error

	sessModel, ok, _ := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, req.SessionId)
	if req.IsContinue {
		// 🎯 继续生成 (Continue Generation): 读取历史，直接让 LLM 顺着末尾 Assistant 内容接续往下写
		if req.SessionId == "" || !ok || sessModel == nil {
			return fmt.Errorf("继续生成请求所对应的会话不存在或未提供有效的 session_id")
		}
		sessionID = req.SessionId

		// 🎯 优先尝试无缝唤醒子 Agent 的 Pause Checkpoint 秒级断点续跑
		subLoopRes, subResumed, subErr := s.trySubAgentCheckpointResume(ctx, &pb.ResumeRequest{
			SessionId:        sessionID,
			EnableRag:        req.EnableRag,
			EnableSkill:      req.EnableSkill,
			EnableMcp:        req.EnableMcp,
			EnableRerank:     req.EnableRerank,
			KbTenantId:       req.KbTenantId,
			KbId:             req.KbId,
			AgentToolOptions: req.AgentToolOptions,
		}, nil, nil, emitter)
		if subErr != nil {
			return subErr
		}
		if subResumed && subLoopRes != nil && subLoopRes.Status == pb.SessionStatus_SS_INTERRUPTED {
			log.Infow(ctx, "stream_completion_sub_agent_interrupted_again", "session_id", sessionID)
			return nil
		}

		messages, err = s.sessionMgr.LoadHistoryForLLM(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("读取历史以继续生成失败: %w", err)
		}

		// 🎯 兼容 OpenAI API 规范：为未写完的 Assistant 补齐 Continuation 提示，保证大模型流畅向下接续
		if len(messages) > 0 && messages[len(messages)-1].Role == openai.ChatMessageRoleAssistant && sessModel.Status == pb.SessionStatus_SS_PAUSED {
			lastContent := messages[len(messages)-1].Content
			contPrompt := fmt.Sprintf("请从你刚才中断的地方接着继续输出，紧接在 '%s' 后面直接写后面的内容，不要重复前面写过的文字。", agentbase.TruncateText(lastContent, 80))
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: contPrompt,
			})
		}
		newMessageStart = len(messages)
	} else {
		// 🎯 检查当前 Session 是否处于 SS_INTERRUPTED。若是且用户发了新 Prompt，说明放弃前次审批中断，自动取消中断并闭合消息链
		if ok && sessModel.Status == pb.SessionStatus_SS_INTERRUPTED {
			log.Infow(ctx, "user_discarded_interrupt_with_new_prompt", "session_id", req.SessionId)
			if cleanErr := s.sessionMgr.CleanOrCancelPendingInterrupts(ctx, req.SessionId); cleanErr != nil {
				log.Warnw(ctx, "clean_pending_interrupts_failed", "error", cleanErr)
			}
		}

		sessionID, err = s.sessionMgr.InitOrCreateSession(ctx, req.SessionId, req.Message)
		if err != nil {
			return err
		}

		messages, newMessageStart, err = s.sessionMgr.PrepareMessagesForCompletion(ctx, sessionID, req.Message)
		if err != nil {
			return err
		}
	}

	// 🎯 立即向前端推送首帧 (包含正式的 session_id)，确保前端立刻同步 sessionID 路由与状态
	emitter(&pb.StreamChunk{
		Event:     pb.StreamEventType_SET_UNSPECIFIED,
		SessionId: sessionID,
		Status:    pb.SessionStatus_SS_RUNNING,
	})

	ag, ok := s.agentRegistry.Get(agent.MainAgentName)
	if !ok {
		return fmt.Errorf("未找到默认 main agent")
	}

	approvedTools := s.sessionMgr.LoadSessionApprovedTools(ctx, sessionID)

	log.Debugw(ctx, "stream_completion_start", "session_id", sessionID, "agent_name", ag.Name(), "model", ag.Model(), "is_continue", req.IsContinue)

	var currentCompressCount int32 = 0
	if sessModel, ok, _ := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID); ok && sessModel != nil {
		currentCompressCount = sessModel.CompressCount
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	s.RegisterActiveCancel(sessionID, cancelRun)
	defer s.UnregisterActiveCancel(sessionID)

	finalRAG, finalSkill, finalMCP, finalRerank := resolveEnableFlags(s.cfg, req.EnableRag, req.EnableSkill, req.EnableMcp, req.EnableRerank)
	agentOpts := parseAgentToolOptions(req.AgentToolOptions)
	runCtx = s.withParentContext(runCtx, sessionID, req.KbTenantId, req.KbId, finalRAG, finalSkill, finalMCP, finalRerank, agentOpts, approvedTools, nil, messages, emitter)
	fetcher := ag.GetStreamFetcher(sessionID, s.openaiChatModel, emitter)
	syncFetcher := ag.GetSyncFetcher(s.openaiChatModel)
	loopRes, err := ag.Run(runCtx, &agentbase.RunOptions{
		SessionID:     sessionID,
		Messages:      messages,
		ApprovedTools: approvedTools,
		Emitter:       emitter,
		Fetcher:       fetcher,
		SyncFetcher:   syncFetcher,
		Compressor:    s.contextCompressor,
		CompressCount: currentCompressCount,
	})

	// 🎯 脱钩落盘：即使客户端中途切断 HTTP 连接，仍使用 5 秒独立的 saveCtx 保证数据 100% 安全入库
	saveCtx, saveCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer saveCancel()

	if err != nil {
		log.Errorw(ctx, "stream_completion_error", "session_id", sessionID, "error", err)
		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_ERROR,
			SessionId: sessionID,
			Status:    pb.SessionStatus_SS_IDLE,
			Error:     &pb.StreamError{Code: 500, Message: err.Error()},
		})
		if loopRes != nil && len(loopRes.Messages) > newMessageStart {
			_ = s.sessionMgr.FinalizeSessionTurn(saveCtx, sessionID, loopRes.Messages[newMessageStart:], nil, pb.SessionStatus_SS_IDLE, nil)
		}
		return err
	}

	var pendingInterrupt *dataBase.NocliInterruptModel
	if loopRes.Status == pb.SessionStatus_SS_INTERRUPTED && len(loopRes.PendingToolCalls) > 0 {
		pendingInterrupt = &dataBase.NocliInterruptModel{
			InterruptID: loopRes.PendingToolCalls[0].InterruptId,
			SessionID:   sessionID,
			Status:      pb.InterruptStatus_IS_PENDING,
			ToolCallID:  loopRes.PendingToolCalls[0].ToolCallId,
			ToolName:    loopRes.PendingToolCalls[0].ToolName,
			Arguments:   loopRes.PendingToolCalls[0].Arguments,
			CreatedAt:   time.Now().Unix(),
		}
	}

	finalStatus := loopRes.Status
	if ctx.Err() != nil || loopRes.Status == pb.SessionStatus_SS_PAUSED {
		finalStatus = pb.SessionStatus_SS_PAUSED
		log.Infow(ctx, "stream_completion_canceled_gracefully", "session_id", sessionID)
		if loopRes != nil && len(loopRes.Messages) > newMessageStart {
			_ = s.sessionMgr.FinalizeSessionTurn(saveCtx, sessionID, loopRes.Messages[newMessageStart:], nil, pb.SessionStatus_SS_PAUSED, nil, loopRes.ToolDurations)
		} else {
			_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(saveCtx, sessionID, pb.SessionStatus_SS_PAUSED)
		}
		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_DONE,
			SessionId: sessionID,
			Status:    pb.SessionStatus_SS_PAUSED,
		})
		return nil

	}

	if err := s.sessionMgr.FinalizeSessionTurn(saveCtx, sessionID, loopRes.Messages[newMessageStart:], pendingInterrupt, finalStatus, loopRes.NewCheckpointMsg, loopRes.ToolDurations); err != nil {
		return err
	}

	return nil
}

// StreamResume 流式恢复执行 (由 Service 层直接传入 StreamEmitter 闭包)
func (s *ChatBiz) StreamResume(ctx context.Context, req *pb.ResumeRequest, emitter agentbase.StreamEmitter) error {
	if emitter == nil {
		emitter = agentbase.NoopStreamEmitter
	}

	approvedTools, rejectedTools, err := s.sessionMgr.ValidateAndPrepareResume(ctx, req)
	if err != nil {
		return err
	}

	messages, err := s.sessionMgr.LoadHistoryForLLM(ctx, req.SessionId)
	if err != nil {
		return fmt.Errorf("加载对话历史失败: %v", err)
	}

	finalRAG, finalSkill, finalMCP, finalRerank := resolveEnableFlags(s.cfg, req.EnableRag, req.EnableSkill, req.EnableMcp, req.EnableRerank)
	agentOpts := parseAgentToolOptions(req.AgentToolOptions)
	ctx = s.withParentContext(ctx, req.SessionId, req.KbTenantId, req.KbId, finalRAG, finalSkill, finalMCP, finalRerank, agentOpts, approvedTools, rejectedTools, messages, emitter)

	// 🎯 优先尝试从子 Agent 专属 Checkpoint 秒级快速恢复执行
	subLoopRes, subResumed, subErr := s.trySubAgentCheckpointResume(ctx, req, approvedTools, rejectedTools, emitter)
	if subErr != nil {
		return subErr
	}
	if subResumed {
		if subLoopRes != nil && subLoopRes.Status == pb.SessionStatus_SS_INTERRUPTED {
			log.Infow(ctx, "stream_resume_sub_agent_interrupted_again", "session_id", req.SessionId)
			return nil
		}
		log.Infow(ctx, "stream_resume_sub_agent_completed_continuing_parent", "session_id", req.SessionId)
		// 重新从 DB 加载包含了子 Agent 总结 ToolResult 的最新 LLM 历史，继续驱动主 Agent
		messages, err = s.sessionMgr.LoadHistoryForLLM(ctx, req.SessionId)
		if err != nil {
			return fmt.Errorf("加载更新后的对话历史失败: %v", err)
		}
	}

	ag, ok := s.agentRegistry.Get(agent.MainAgentName)
	if !ok {
		return fmt.Errorf("未找到默认 main agent")
	}

	newMessageStart := len(messages)

	log.Debugw(ctx, "stream_resume_start", "session_id", req.SessionId, "agent_name", ag.Name(), "model", ag.Model())

	var currentCompressCount int32 = 0
	if sessModel, ok, _ := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, req.SessionId); ok && sessModel != nil {
		currentCompressCount = sessModel.CompressCount
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	s.RegisterActiveCancel(req.SessionId, cancelRun)
	defer s.UnregisterActiveCancel(req.SessionId)

	fetcher := ag.GetStreamFetcher(req.SessionId, s.openaiChatModel, emitter)
	syncFetcher := ag.GetSyncFetcher(s.openaiChatModel)
	loopRes, err := ag.Run(runCtx, &agentbase.RunOptions{
		SessionID:     req.SessionId,
		Messages:      messages,
		ApprovedTools: approvedTools,
		RejectedTools: rejectedTools,
		Emitter:       emitter,
		Fetcher:       fetcher,
		SyncFetcher:   syncFetcher,
		Compressor:    s.contextCompressor,
		CompressCount: currentCompressCount,
	})

	// 🎯 脱钩落盘：即使客户端中途切断 HTTP 连接，仍使用 5 秒独立的 saveCtx 保证数据 100% 安全入库
	saveCtx, saveCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer saveCancel()

	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil || strings.Contains(err.Error(), "context canceled") {
			log.Infow(ctx, "stream_resume_canceled_gracefully", "session_id", req.SessionId)
			if loopRes != nil && len(loopRes.Messages) > newMessageStart {
				_ = s.sessionMgr.FinalizeSessionTurn(saveCtx, req.SessionId, loopRes.Messages[newMessageStart:], nil, pb.SessionStatus_SS_PAUSED, nil, loopRes.ToolDurations)
			} else {
				_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(saveCtx, req.SessionId, pb.SessionStatus_SS_PAUSED)
			}
			emitter(&pb.StreamChunk{
				Event:     pb.StreamEventType_SET_DONE,
				SessionId: req.SessionId,
				Status:    pb.SessionStatus_SS_PAUSED,
			})
			return nil
		}

		log.Errorw(ctx, "stream_resume_error", "session_id", req.SessionId, "error", err)
		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_ERROR,
			SessionId: req.SessionId,
			Status:    pb.SessionStatus_SS_IDLE,
			Error:     &pb.StreamError{Code: 500, Message: err.Error()},
		})
		if loopRes != nil && len(loopRes.Messages) > newMessageStart {
			_ = s.sessionMgr.FinalizeSessionTurn(saveCtx, req.SessionId, loopRes.Messages[newMessageStart:], nil, pb.SessionStatus_SS_IDLE, nil)
		}
		return err
	}

	var pendingInterrupt *dataBase.NocliInterruptModel
	if loopRes.Status == pb.SessionStatus_SS_INTERRUPTED && len(loopRes.PendingToolCalls) > 0 {
		pendingInterrupt = &dataBase.NocliInterruptModel{
			InterruptID: loopRes.PendingToolCalls[0].InterruptId,
			SessionID:   req.SessionId,
			Status:      pb.InterruptStatus_IS_PENDING,
			ToolCallID:  loopRes.PendingToolCalls[0].ToolCallId,
			ToolName:    loopRes.PendingToolCalls[0].ToolName,
			Arguments:   loopRes.PendingToolCalls[0].Arguments,
			CreatedAt:   time.Now().Unix(),
		}
	}

	finalStatus := loopRes.Status
	if ctx.Err() != nil || loopRes.Status == pb.SessionStatus_SS_PAUSED {
		finalStatus = pb.SessionStatus_SS_PAUSED
	}

	if err := s.sessionMgr.FinalizeSessionTurn(saveCtx, req.SessionId, loopRes.Messages[newMessageStart:], pendingInterrupt, finalStatus, loopRes.NewCheckpointMsg, loopRes.ToolDurations); err != nil {
		return err
	}

	return nil
}

// trySubAgentCheckpointResume 尝试利用子 Agent Checkpoint 快速秒级恢复执行
func (s *ChatBiz) trySubAgentCheckpointResume(
	ctx context.Context,
	req *pb.ResumeRequest,
	approvedTools map[string]bool,
	rejectedTools map[string]string,
	emitter agentbase.StreamEmitter,
) (*agentbase.LoopResult, bool, error) {
	cp, ok := s.sessionMgr.GetSubAgentCheckpoint(req.SessionId)
	if !ok || cp == nil {
		return nil, false, nil
	}

	// 🎯 校验中断事件 ID 匹配，确保抓取的 Checkpoint 对应当前审批的项目
	if req.InterruptId != "" && cp.InterruptID != "" && cp.InterruptID != req.InterruptId {
		log.Warnw(ctx, "sub_agent_checkpoint_interrupt_id_mismatch",
			"session_id", req.SessionId,
			"req_interrupt_id", req.InterruptId,
			"cp_interrupt_id", cp.InterruptID,
		)
		return nil, false, nil
	}

	targetAgent, found := s.agentRegistry.Get(cp.TargetAgentName)
	if !found || targetAgent == nil {
		return nil, false, nil
	}

	log.Infow(ctx, "sub_agent_checkpoint_fast_resume_start",
		"session_id", req.SessionId,
		"target_agent", cp.TargetAgentName,
		"interrupt_id", cp.InterruptID,
		"sub_messages_count", len(cp.SubMessages),
	)

	// 清理当前已领取的 Checkpoint
	s.sessionMgr.ClearSubAgentCheckpoint(req.SessionId)

	// 🎯 恢复上下文配置：优先使用 req 传入参数，未提供时降级使用 Checkpoint 归档配置
	finalRAG := req.EnableRag || cp.EnableRAG
	finalSkill := req.EnableSkill || cp.EnableSkill
	finalMCP := req.EnableMcp || cp.EnableMCP
	finalRerank := req.EnableRerank || cp.EnableRerank
	kbTenantID := req.KbTenantId
	if kbTenantID == "" {
		kbTenantID = cp.KBTenantID
	}
	kbID := req.KbId
	if kbID == "" {
		kbID = cp.KBID
	}
	agentOpts := parseAgentToolOptions(req.AgentToolOptions)
	if req.AgentToolOptions == nil {
		agentOpts = agentbase.AgentToolOptions{
			PassFullContextToSubAgent: cp.AgentToolOptions.PassFullContextToSubAgent,
			ReturnFullContextToParent: cp.AgentToolOptions.ReturnFullContextToParent,
			StreamSubAgentExecution:   cp.AgentToolOptions.StreamSubAgentExecution,
		}
	}

	if approvedTools == nil {
		approvedTools = make(map[string]bool)
	}
	for k, v := range cp.ApprovedTools {
		approvedTools[k] = v
	}

	if rejectedTools == nil {
		rejectedTools = make(map[string]string)
	}
	for k, v := range cp.RejectedTools {
		rejectedTools[k] = v
	}

	// 🎯 确保 withParentContext 被注入 context
	ctx = s.withParentContext(ctx, req.SessionId, kbTenantID, kbID, finalRAG, finalSkill, finalMCP, finalRerank, agentOpts, approvedTools, rejectedTools, cp.SubMessages, emitter)

	subEmitter := func(chunk *pb.StreamChunk) {
		if chunk != nil {
			chunk.AgentName = targetAgent.Name()
		}
		if emitter != nil {
			emitter(chunk)
		}
	}

	fetcher := targetAgent.GetStreamFetcher(req.SessionId, s.openaiChatModel, subEmitter)
	syncFetcher := targetAgent.GetSyncFetcher(s.openaiChatModel)

	subResumeStart := time.Now()

	loopRes, err := targetAgent.Run(ctx, &agentbase.RunOptions{
		SessionID:     req.SessionId,
		Messages:      cp.SubMessages,
		ApprovedTools: approvedTools,
		RejectedTools: rejectedTools,
		Emitter:       subEmitter,
		Fetcher:       fetcher,
		SyncFetcher:   syncFetcher,
	})

	if err != nil {
		return nil, true, fmt.Errorf("子 Agent Checkpoint 快速恢复执行失败: %v", err)
	}

	subDurationMS := time.Since(subResumeStart).Milliseconds()
	if subDurationMS <= 0 {
		subDurationMS = 1
	}

	if loopRes.ToolDurations == nil {
		loopRes.ToolDurations = make(map[string]int64)
	}
	if parentToolCallID := cp.ParentToolCallID; parentToolCallID != "" {
		loopRes.ToolDurations[parentToolCallID] = subDurationMS
	}

	// 1. 如果子 Agent 再次触发了授权中断 (如第 2 个高危命令)
	if loopRes.Status == pb.SessionStatus_SS_INTERRUPTED {
		var pendingCall *pb.PendingToolCall
		if len(loopRes.PendingToolCalls) > 0 {
			pendingCall = loopRes.PendingToolCalls[0]
			if pendingCall.AgentName == "" {
				pendingCall.AgentName = targetAgent.Name()
			}
		}

		// 存入新的 Checkpoint 快照 (保留 ParentToolCallID 和配置)
		newCp := &agentbase.SubAgentCheckpoint{
			SessionID:        req.SessionId,
			InterruptID:      pendingCall.GetInterruptId(),
			TargetAgentName:  targetAgent.Name(),
			ParentToolCallID: cp.ParentToolCallID,
			SubMessages:      loopRes.Messages,
			PendingToolCall:  pendingCall,
			AgentToolOptions: agentOpts,
			KBTenantID:       kbTenantID,
			KBID:             kbID,
			EnableRAG:        finalRAG,
			EnableSkill:      finalSkill,
			EnableMCP:        finalMCP,
			EnableRerank:     finalRerank,
			ApprovedTools:    approvedTools,
			RejectedTools:    rejectedTools,
			CreatedAt:        time.Now().Unix(),
		}
		s.sessionMgr.SaveSubAgentCheckpoint(req.SessionId, newCp)

		var pendingInterrupt *dataBase.NocliInterruptModel
		if pendingCall != nil {
			pendingInterrupt = &dataBase.NocliInterruptModel{
				InterruptID: pendingCall.InterruptId,
				SessionID:   req.SessionId,
				Status:      pb.InterruptStatus_IS_PENDING,
				ToolCallID:  pendingCall.ToolCallId,
				ToolName:    pendingCall.ToolName,
				Arguments:   pendingCall.Arguments,
				CreatedAt:   time.Now().Unix(),
			}
		}

		if err := s.sessionMgr.FinalizeSessionTurn(ctx, req.SessionId, nil, pendingInterrupt, pb.SessionStatus_SS_INTERRUPTED, nil); err != nil {
			return nil, true, err
		}
		return loopRes, true, nil
	}

	// 2. 如果子 Agent 执行完成，构造最终总结消息并补齐主 Agent 会话中关联的 tool call
	toolResult := fmt.Sprintf("【子 Agent (%s) 独立执行总结】:\n%s", targetAgent.Name(), loopRes.Reply)

	parentToolCallID := cp.ParentToolCallID

	toolMsg := openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Content:    toolResult,
		ToolCallID: parentToolCallID,
		Name:       agent.MainAgentName,
	}

	executedMsgs := []openai.ChatCompletionMessage{toolMsg}
	if agentOpts.ReturnFullContextToParent {
		for _, m := range loopRes.Messages {
			if m.Role == openai.ChatMessageRoleSystem {
				continue
			}
			m.Name = targetAgent.Name()
			if m.Role == openai.ChatMessageRoleUser {
				m.Role = openai.ChatMessageRoleAssistant
				m.Content = fmt.Sprintf("📋 【委派任务指令】: %s", m.Content)
			}
			executedMsgs = append(executedMsgs, m)
		}
	}

	// 将补齐的 Tool 消息落盘保存 (置 SessionStatus 为 RUNNING，以便后续 Main Agent 继续执行)
	if err := s.sessionMgr.FinalizeSessionTurn(ctx, req.SessionId, executedMsgs, nil, pb.SessionStatus_SS_RUNNING, nil, loopRes.ToolDurations); err != nil {
		return nil, true, err
	}

	// 🎯 推送 Parent Tool Result Chunk 给前端 (让父 Agent 的 delegate_to_<subagent> 工具框状态置为 completed)
	if emitter != nil && parentToolCallID != "" {
		emitter(&pb.StreamChunk{
			Event:      pb.StreamEventType_SET_TOOL_RESULT,
			Role:       openai.ChatMessageRoleTool,
			AgentName:  agent.MainAgentName,
			SessionId:  req.SessionId,
			Status:     pb.SessionStatus_SS_RUNNING,
			DurationMs: subDurationMS,
			ToolInfo: &pb.StreamToolInfo{
				ToolCallId:    parentToolCallID,
				ToolName:      fmt.Sprintf("delegate_to_%s", targetAgent.Name()),
				ResultPreview: agentbase.TruncateText(toolResult, 200),
				DurationMs:    subDurationMS,
			},
		})
	}

	return loopRes, true, nil
}
