package nocli

import (
	"context"
	"fmt"
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

	sessionID, err := s.sessionMgr.InitOrCreateSession(ctx, req.SessionId, req.Message)
	if err != nil {
		return err
	}

	messages, newMessageStart, err := s.sessionMgr.PrepareMessagesForCompletion(ctx, sessionID, req.Message)
	if err != nil {
		return err
	}

	ag, ok := s.agentRegistry.Get(agent.MainAgentName)
	if !ok {
		return fmt.Errorf("未找到默认 main agent")
	}

	approvedTools := s.sessionMgr.LoadSessionApprovedTools(ctx, sessionID)

	log.Debugw(ctx, "stream_completion_start", "session_id", sessionID, "agent_name", ag.Name(), "model", ag.Model())

	var currentCompressCount int32 = 0
	if sessModel, ok, _ := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID); ok && sessModel != nil {
		currentCompressCount = sessModel.CompressCount
	}

	finalRAG, finalSkill, finalMCP, finalRerank := resolveEnableFlags(s.cfg, req.EnableRag, req.EnableSkill, req.EnableMcp, req.EnableRerank)
	agentOpts := parseAgentToolOptions(req.AgentToolOptions)
	ctx = s.withParentContext(ctx, sessionID, req.KbTenantId, req.KbId, finalRAG, finalSkill, finalMCP, finalRerank, agentOpts, messages, emitter)
	fetcher := ag.GetStreamFetcher(sessionID, s.openaiChatModel, emitter)
	syncFetcher := ag.GetSyncFetcher(s.openaiChatModel)
	loopRes, err := ag.Run(ctx, &agentbase.RunOptions{
		SessionID:     sessionID,
		Messages:      messages,
		ApprovedTools: approvedTools,
		Emitter:       emitter,
		Fetcher:       fetcher,
		SyncFetcher:   syncFetcher,
		Compressor:    s.contextCompressor,
		CompressCount: currentCompressCount,
	})
	if err != nil {
		log.Errorw(ctx, "stream_completion_error", "session_id", sessionID, "error", err)
		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_ERROR,
			SessionId: sessionID,
			Status:    pb.SessionStatus_SS_IDLE,
			Error:     &pb.StreamError{Code: 500, Message: err.Error()},
		})
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

	if err := s.sessionMgr.FinalizeSessionTurn(ctx, sessionID, loopRes.Messages[newMessageStart:], pendingInterrupt, loopRes.Status, loopRes.NewCheckpointMsg, loopRes.ToolDurations); err != nil {
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
	ctx = s.withParentContext(ctx, req.SessionId, req.KbTenantId, req.KbId, finalRAG, finalSkill, finalMCP, finalRerank, agentOpts, messages, emitter)

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

	fetcher := ag.GetStreamFetcher(req.SessionId, s.openaiChatModel, emitter)
	syncFetcher := ag.GetSyncFetcher(s.openaiChatModel)
	loopRes, err := ag.Run(ctx, &agentbase.RunOptions{
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
	if err != nil {
		log.Errorw(ctx, "stream_resume_error", "session_id", req.SessionId, "error", err)
		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_ERROR,
			SessionId: req.SessionId,
			Status:    pb.SessionStatus_SS_IDLE,
			Error:     &pb.StreamError{Code: 500, Message: err.Error()},
		})
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

	if err := s.sessionMgr.FinalizeSessionTurn(ctx, req.SessionId, loopRes.Messages[newMessageStart:], pendingInterrupt, loopRes.Status, loopRes.NewCheckpointMsg, loopRes.ToolDurations); err != nil {
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

	// 🎯 确保 withParentContext 被注入 context
	ctx = s.withParentContext(ctx, req.SessionId, kbTenantID, kbID, finalRAG, finalSkill, finalMCP, finalRerank, agentOpts, cp.SubMessages, emitter)

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
