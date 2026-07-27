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

	ctx = s.withParentContext(ctx, sessionID, &messages, emitter)
	fetcher := ag.GetStreamFetcher(sessionID, s.openaiChatModel, emitter)
	loopRes, err := ag.Run(ctx, &agentbase.RunOptions{
		SessionID:     sessionID,
		Messages:      messages,
		ApprovedTools: approvedTools,
		Emitter:       emitter,
		Fetcher:       fetcher,
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

	if err := s.sessionMgr.FinalizeSessionTurn(ctx, sessionID, loopRes.Messages[newMessageStart:], pendingInterrupt, loopRes.Status); err != nil {
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

	messages, err := s.sessionMgr.LoadHistory(ctx, req.SessionId)
	if err != nil {
		return fmt.Errorf("加载对话历史失败: %v", err)
	}

	ag, ok := s.agentRegistry.Get(agent.MainAgentName)
	if !ok {
		return fmt.Errorf("未找到默认 main agent")
	}

	newMessageStart := len(messages)

	log.Debugw(ctx, "stream_resume_start", "session_id", req.SessionId, "agent_name", ag.Name(), "model", ag.Model())

	ctx = s.withParentContext(ctx, req.SessionId, &messages, emitter)
	fetcher := ag.GetStreamFetcher(req.SessionId, s.openaiChatModel, emitter)
	loopRes, err := ag.Run(ctx, &agentbase.RunOptions{
		SessionID:     req.SessionId,
		Messages:      messages,
		ApprovedTools: approvedTools,
		RejectedTools: rejectedTools,
		Emitter:       emitter,
		Fetcher:       fetcher,
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

	if err := s.sessionMgr.FinalizeSessionTurn(ctx, req.SessionId, loopRes.Messages[newMessageStart:], pendingInterrupt, loopRes.Status); err != nil {
		return err
	}

	return nil
}
