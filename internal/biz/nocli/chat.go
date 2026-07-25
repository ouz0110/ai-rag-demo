package nocli

import (
	"context"
	"fmt"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/openai/agent"
	agentbase "ai-rag-demo/internal/biz/nocli/openai/agent/base"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"ai-rag-demo/internal/biz/nocli/session"
	"ai-rag-demo/internal/cache"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/skill"
)

type ChatBiz struct {
	cache           *cache.Cache
	openaiChatModel *chatmodel.ChatModel
	agentRegistry   *agent.Registry
	skillManager    *skill.Manager
	sessionMgr      *session.SessionManager
	cfg             *conf.Config
	allDb           *data.DB
}

func NewChatBiz(
	cache *cache.Cache,
	openaiChatModel *chatmodel.ChatModel,
	cfg *conf.Config,
	allDb *data.DB,
) *ChatBiz {
	var skillsDir string
	if cfg != nil && cfg.Source.Skill != nil {
		skillsDir = cfg.Source.Skill.Path
	}
	skillReg := skill.NewRegistry(skillsDir)
	if err := skillReg.Scan(); err != nil {
		log.Errorw(context.Background(), "skill_scan_error", "path", skillsDir, "error", err)
	}

	agentReg := agent.NewRegistry(cfg, openaiChatModel)
	skillMgr := skill.NewManager(skillReg)

	sessionMgr := session.NewSessionManager(allDb, cfg, agentReg, skillMgr)

	return &ChatBiz{
		cache:           cache,
		openaiChatModel: openaiChatModel,
		agentRegistry:   agentReg,
		skillManager:    skillMgr,
		sessionMgr:      sessionMgr,
		cfg:             cfg,
		allDb:           allDb,
	}
}

func (s *ChatBiz) Completion(ctx context.Context, req *pb.CompletionRequest) (*pb.StreamChunk, error) {
	sessionID, err := s.sessionMgr.InitOrCreateSession(ctx, req.SessionId, req.Message)
	if err != nil {
		return nil, err
	}

	messages, newMessageStart, err := s.sessionMgr.PrepareMessagesForCompletion(ctx, sessionID, req.Message)
	if err != nil {
		return nil, err
	}

	ag, ok := s.agentRegistry.Get(agent.MainAgentName)
	if !ok {
		return nil, fmt.Errorf("未找到默认 main agent")
	}

	approvedTools := s.sessionMgr.LoadSessionApprovedTools(ctx, sessionID)

	ctx = context.WithValue(ctx, "parent_session_id", sessionID)
	ctx = context.WithValue(ctx, "parent_messages", messages)
	start := time.Now()
	fetcher := ag.GetSyncFetcher(s.openaiChatModel)
	loopRes, err := ag.Run(ctx, &agentbase.RunOptions{
		SessionID:     sessionID,
		Messages:      messages,
		ApprovedTools: approvedTools,
		Fetcher:       fetcher,
	})
	duration := time.Since(start)

	if err != nil {
		log.Errorw(ctx, "completion_error", "session_id", sessionID, "duration_ms", duration.Milliseconds(), "error", err)
		return &pb.StreamChunk{
			Event:     pb.StreamEventType_SET_ERROR,
			SessionId: sessionID,
			Status:    pb.SessionStatus_SS_IDLE,
			Error:     &pb.StreamError{Code: 500, Message: err.Error()},
		}, nil
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
		return nil, err
	}

	log.Debugw(ctx, "completion_success", "session_id", sessionID, "status", loopRes.Status, "duration_ms", duration.Milliseconds())

	if loopRes.Status == pb.SessionStatus_SS_INTERRUPTED {
		return &pb.StreamChunk{
			Event:            pb.StreamEventType_SET_INTERRUPT,
			SessionId:        sessionID,
			Status:           pb.SessionStatus_SS_INTERRUPTED,
			PendingToolCalls: loopRes.PendingToolCalls,
		}, nil
	}

	return &pb.StreamChunk{
		Event:     pb.StreamEventType_SET_DONE,
		SessionId: sessionID,
		Status:    pb.SessionStatus_SS_IDLE,
		Text:      loopRes.Reply,
	}, nil
}

func (s *ChatBiz) Resume(ctx context.Context, req *pb.ResumeRequest) (*pb.StreamChunk, error) {
	approvedTools, rejectedTools, err := s.sessionMgr.ValidateAndPrepareResume(ctx, req)
	if err != nil {
		return nil, err
	}

	messages, err := s.sessionMgr.LoadHistory(ctx, req.SessionId)
	if err != nil {
		return nil, fmt.Errorf("加载对话历史失败: %v", err)
	}

	ag, ok := s.agentRegistry.Get(agent.MainAgentName)
	if !ok {
		return nil, fmt.Errorf("未找到默认 main agent")
	}

	newMessageStart := len(messages)

	ctx = context.WithValue(ctx, "parent_session_id", req.SessionId)
	ctx = context.WithValue(ctx, "parent_messages", messages)
	fetcher := ag.GetSyncFetcher(s.openaiChatModel)
	loopRes, err := ag.Run(ctx, &agentbase.RunOptions{
		SessionID:     req.SessionId,
		Messages:      messages,
		ApprovedTools: approvedTools,
		RejectedTools: rejectedTools,
		Fetcher:       fetcher,
	})
	if err != nil {
		return nil, err
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
		return nil, err
	}

	if loopRes.Status == pb.SessionStatus_SS_INTERRUPTED {
		return &pb.StreamChunk{
			Event:            pb.StreamEventType_SET_INTERRUPT,
			SessionId:        req.SessionId,
			Status:           pb.SessionStatus_SS_INTERRUPTED,
			PendingToolCalls: loopRes.PendingToolCalls,
		}, nil
	}

	return &pb.StreamChunk{
		Event:     pb.StreamEventType_SET_DONE,
		SessionId: req.SessionId,
		Status:    pb.SessionStatus_SS_IDLE,
		Text:      loopRes.Reply,
	}, nil
}

// ListSessions 会话列表接口
func (s *ChatBiz) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	return s.sessionMgr.ListSessions(ctx, req)
}

// DeleteSession 删除会话接口
func (s *ChatBiz) DeleteSession(ctx context.Context, req *pb.DeleteSessionRequest) (*pb.DeleteSessionResponse, error) {
	if req == nil || req.SessionId == "" {
		return nil, fmt.Errorf("session_id 不能为空")
	}
	if err := s.sessionMgr.DeleteSession(ctx, req.SessionId); err != nil {
		return nil, err
	}
	return &pb.DeleteSessionResponse{
		Success:   true,
		SessionId: req.SessionId,
	}, nil
}

// GetSessionHistory 获取会话历史记录接口
func (s *ChatBiz) GetSessionHistory(ctx context.Context, req *pb.GetSessionHistoryRequest) (*pb.GetSessionHistoryResponse, error) {
	if req == nil || req.SessionId == "" {
		return nil, fmt.Errorf("session_id 不能为空")
	}
	return s.sessionMgr.GetSessionHistory(ctx, req)
}
