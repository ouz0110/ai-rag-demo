package nocli

import (
	"context"
	"fmt"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/openai/agent"
	agentbase "ai-rag-demo/internal/biz/nocli/openai/agent/base"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"ai-rag-demo/internal/biz/nocli/openai/compressor"
	"ai-rag-demo/internal/biz/nocli/session"
	"ai-rag-demo/internal/biz/nocli/vector"
	"ai-rag-demo/internal/cache"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/external/mcp"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/skill"

	openai "github.com/sashabaranov/go-openai"
)

type ChatBiz struct {
	cache             *cache.Cache
	openaiChatModel   *chatmodel.ChatModel
	agentRegistry     *agent.Registry
	skillManager      *skill.Manager
	mcpManager        mcp.Manager
	sessionMgr        *session.SessionManager
	vectorEngine      *vector.VectorEngine
	cfg               *conf.Config
	allDb             *data.DB
	contextCompressor compressor.ICompressor
}

func NewChatBiz(
	cache *cache.Cache,
	openaiChatModel *chatmodel.ChatModel,
	cfg *conf.Config,
	allDb *data.DB,
	vectorEngine *vector.VectorEngine,
	mcpMgr mcp.Manager,
) *ChatBiz {
	var enableSkill bool
	var skillsDir string
	if cfg != nil && cfg.Source.Skill != nil {
		enableSkill = cfg.Source.Skill.Enable
		skillsDir = cfg.Source.Skill.Path
	}
	skillReg := skill.NewRegistry(skillsDir)
	if enableSkill {
		if err := skillReg.Scan(); err != nil {
			log.Errorw(context.Background(), "skill_scan_error", "path", skillsDir, "error", err)
		}
	}

	skillMgr := skill.NewManager(skillReg)
	agentReg := agent.NewRegistry(cfg, openaiChatModel, skillMgr, mcpMgr, vectorEngine)

	sessionMgr := session.NewSessionManager(allDb, cfg, agentReg)

	var compressCfg *conf.OpenAIContextCompressConfig
	if cfg != nil && cfg.Source.OpenAI != nil {
		compressCfg = cfg.Source.OpenAI.ContextCompress
	}
	var contextCompressor compressor.ICompressor
	if compressCfg != nil {
		contextCompressor = compressor.NewContextCompressor(compressCfg)
	}

	return &ChatBiz{
		cache:             cache,
		openaiChatModel:   openaiChatModel,
		agentRegistry:     agentReg,
		skillManager:      skillMgr,
		mcpManager:        mcpMgr,
		sessionMgr:        sessionMgr,
		vectorEngine:      vectorEngine,
		cfg:               cfg,
		allDb:             allDb,
		contextCompressor: contextCompressor,
	}
}

func resolveEnableFlags(cfg *conf.Config, reqRAG, reqSkill, reqMCP, reqRerank bool) (bool, bool, bool, bool) {
	finalRAG := false
	if cfg != nil && cfg.Source.RAG != nil && cfg.Source.RAG.Enable {
		finalRAG = reqRAG
	}

	finalSkill := false
	if cfg != nil && cfg.Source.Skill != nil && cfg.Source.Skill.Enable {
		finalSkill = reqSkill
	}

	finalMCP := false
	if cfg != nil && cfg.Source.MCP != nil && cfg.Source.MCP.Enable {
		finalMCP = reqMCP
	}

	finalRerank := false
	if cfg != nil && cfg.Source.RAG != nil && cfg.Source.RAG.Enable && cfg.Source.RAG.Rerank != nil && cfg.Source.RAG.Rerank.Enable {
		finalRerank = reqRerank
	}

	return finalRAG, finalSkill, finalMCP, finalRerank
}

func (s *ChatBiz) getKBInfo(ctx context.Context, tenantID, kbID string) (string, string) {
	if s.allDb == nil || s.allDb.Rag == nil {
		return "", ""
	}
	if kbID != "" {
		kb, err := s.allDb.Rag.KBRepo.GetKnowledgeBaseByID(ctx, tenantID, kbID)
		if err == nil && kb != nil {
			return kb.Name, kb.Description
		}
	}
	defaultKB, err := s.allDb.Rag.KBRepo.GetDefaultKnowledgeBase(ctx, tenantID)
	if err == nil && defaultKB != nil {
		return defaultKB.Name, defaultKB.Description
	}
	return "", ""
}

func parseAgentToolOptions(pbOpts *pb.AgentToolOptions) agentbase.AgentToolOptions {
	if pbOpts == nil {
		return agentbase.AgentToolOptions{
			PassFullContextToSubAgent: false,
			ReturnFullContextToParent: false,
			StreamSubAgentExecution:   true,
		}
	}
	return agentbase.AgentToolOptions{
		PassFullContextToSubAgent: pbOpts.PassFullContextToSubAgent,
		ReturnFullContextToParent: pbOpts.ReturnFullContextToParent,
		StreamSubAgentExecution:   pbOpts.StreamSubAgentExecution,
	}
}

func (s *ChatBiz) withParentContext(ctx context.Context, sessionID, kbTenantID, kbID string, enableRAG, enableSkill, enableMCP, enableRerank bool, agentOpts agentbase.AgentToolOptions, messages []openai.ChatCompletionMessage, emitter agentbase.StreamEmitter) context.Context {
	if kbTenantID == "" {
		kbTenantID = vector.DefaultTenantID
	}
	if kbID == "" {
		kbID = vector.DefaultKBID
	}

	var kbName, kbDesc string
	if enableRAG {
		kbName, kbDesc = s.getKBInfo(ctx, kbTenantID, kbID)
	}

	subBuffer := make([]openai.ChatCompletionMessage, 0)
	var pendingCall *pb.PendingToolCall
	pc := &agentbase.ParentContext{
		SessionID:        sessionID,
		KBTenantID:       kbTenantID,
		KBID:             kbID,
		KBName:           kbName,
		KBDescription:    kbDesc,
		EnableRAG:        enableRAG,
		EnableSkill:      enableSkill,
		EnableMCP:        enableMCP,
		EnableRerank:     enableRerank,
		AgentToolOptions: agentOpts,
		Messages:         messages,
		SubMsgBuffer:     &subBuffer,
		PendingToolCall:  &pendingCall,
		Appender: func(msgs []openai.ChatCompletionMessage) {
			subBuffer = append(subBuffer, msgs...)
		},
	}
	if emitter != nil {
		pc.Emitter = emitter
	}
	parentCtx := pc.Inject(ctx)
	parentCtx = context.WithValue(parentCtx, agentbase.SubAgentCheckpointSaverKey, func(cp *agentbase.SubAgentCheckpoint) {
		s.sessionMgr.SaveSubAgentCheckpoint(sessionID, cp)
	})
	return parentCtx
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

	var currentCompressCount int32 = 0
	if sessModel, ok, _ := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID); ok && sessModel != nil {
		currentCompressCount = sessModel.CompressCount
	}

	finalRAG, finalSkill, finalMCP, finalRerank := resolveEnableFlags(s.cfg, req.EnableRag, req.EnableSkill, req.EnableMcp, req.EnableRerank)
	agentOpts := parseAgentToolOptions(req.AgentToolOptions)
	ctx = s.withParentContext(ctx, sessionID, req.KbTenantId, req.KbId, finalRAG, finalSkill, finalMCP, finalRerank, agentOpts, messages, nil)
	start := time.Now()
	fetcher := ag.GetSyncFetcher(s.openaiChatModel)
	loopRes, err := ag.Run(ctx, &agentbase.RunOptions{
		SessionID:     sessionID,
		Messages:      messages,
		ApprovedTools: approvedTools,
		Fetcher:       fetcher,
		SyncFetcher:   fetcher,
		Compressor:    s.contextCompressor,
		CompressCount: currentCompressCount,
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

	if err := s.sessionMgr.FinalizeSessionTurn(ctx, sessionID, loopRes.Messages[newMessageStart:], pendingInterrupt, loopRes.Status, loopRes.NewCheckpointMsg, loopRes.ToolDurations); err != nil {
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

	messages, err := s.sessionMgr.LoadHistoryForLLM(ctx, req.SessionId)
	if err != nil {
		return nil, fmt.Errorf("加载对话历史失败: %v", err)
	}

	finalRAG, finalSkill, finalMCP, finalRerank := resolveEnableFlags(s.cfg, req.EnableRag, req.EnableSkill, req.EnableMcp, req.EnableRerank)
	agentOpts := parseAgentToolOptions(req.AgentToolOptions)
	ctx = s.withParentContext(ctx, req.SessionId, req.KbTenantId, req.KbId, finalRAG, finalSkill, finalMCP, finalRerank, agentOpts, messages, nil)

	// 🎯 优先尝试从子 Agent 专属 Checkpoint 秒级快速恢复执行
	subLoopRes, subResumed, subErr := s.trySubAgentCheckpointResume(ctx, req, approvedTools, rejectedTools, nil)
	if subErr != nil {
		return nil, subErr
	}
	if subResumed {
		if subLoopRes != nil && subLoopRes.Status == pb.SessionStatus_SS_INTERRUPTED {
			return &pb.StreamChunk{
				Event:            pb.StreamEventType_SET_INTERRUPT,
				SessionId:        req.SessionId,
				Status:           pb.SessionStatus_SS_INTERRUPTED,
				PendingToolCalls: subLoopRes.PendingToolCalls,
			}, nil
		}
		// 重新从 DB 加载包含了子 Agent 总结 ToolResult 的最新 LLM 历史，继续驱动主 Agent
		messages, err = s.sessionMgr.LoadHistoryForLLM(ctx, req.SessionId)
		if err != nil {
			return nil, fmt.Errorf("加载更新后的对话历史失败: %v", err)
		}
	}

	ag, ok := s.agentRegistry.Get(agent.MainAgentName)
	if !ok {
		return nil, fmt.Errorf("未找到默认 main agent")
	}

	newMessageStart := len(messages)

	var currentCompressCount int32 = 0
	if sessModel, ok, _ := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, req.SessionId); ok && sessModel != nil {
		currentCompressCount = sessModel.CompressCount
	}
	fetcher := ag.GetSyncFetcher(s.openaiChatModel)
	loopRes, err := ag.Run(ctx, &agentbase.RunOptions{
		SessionID:     req.SessionId,
		Messages:      messages,
		ApprovedTools: approvedTools,
		RejectedTools: rejectedTools,
		Fetcher:       fetcher,
		SyncFetcher:   fetcher,
		Compressor:    s.contextCompressor,
		CompressCount: currentCompressCount,
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

	if err := s.sessionMgr.FinalizeSessionTurn(ctx, req.SessionId, loopRes.Messages[newMessageStart:], pendingInterrupt, loopRes.Status, loopRes.NewCheckpointMsg, loopRes.ToolDurations); err != nil {
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
