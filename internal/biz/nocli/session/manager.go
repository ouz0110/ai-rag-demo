package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/openai/agent"
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/utils"

	openai "github.com/sashabaranov/go-openai"
)

type SessionManager struct {
	allDb         *data.DB
	cfg           *conf.Config
	agentRegistry *agent.Registry
}

func NewSessionManager(
	allDb *data.DB,
	cfg *conf.Config,
	agentRegistry *agent.Registry,
) *SessionManager {
	return &SessionManager{
		allDb:         allDb,
		cfg:           cfg,
		agentRegistry: agentRegistry,
	}
}

// InitOrCreateSession 初始化或加载会话，负责新建会话与 SystemPrompt 的安全落盘
func (m *SessionManager) InitOrCreateSession(ctx context.Context, sessionID, userMsg string) (string, error) {
	baseAgentDir := "./workspace/agent"
	if m.cfg != nil && m.cfg.Source.Nocli != nil && m.cfg.Source.Nocli.WorkDir != "" {
		baseAgentDir = m.cfg.Source.Nocli.WorkDir
	}

	_, user := common.UserFromContext(ctx)
	agentWorkDir, err := common.GetStrictUserAgentWorkDir(ctx, baseAgentDir)
	if err != nil {
		return "", fmt.Errorf("初始化用户 Agent 工作空间失败: %w", err)
	}

	ag, ok := m.agentRegistry.Get("main")
	if !ok {
		return "", fmt.Errorf("未找到默认 main agent")
	}
	systemPrompt := ag.SystemPrompt(agentWorkDir)
	now := time.Now().Unix()

	if sessionID == "" {
		sessionID = utils.NewUUID()
		sess := &dataBase.NocliSessionModel{
			SessionID: sessionID,
			Openid:    user.Openid,
			Name:      TruncateText(userMsg, 30),
			Status:    pb.SessionStatus_SS_RUNNING,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := m.allDb.Base.NocliSessionRepo.Create(ctx, sess); err != nil {
			return "", fmt.Errorf("创建会话失败: %v", err)
		}
		log.Debugw(ctx, "session_created", "session_id", sessionID)

		sysMsg, _ := utils.JSONMarshal(openai.ChatCompletionMessage{
			Role: "system", Content: systemPrompt,
		})
		if err := m.allDb.Base.NocliMessageRepo.Create(ctx, &dataBase.NocliMessageModel{
			SessionID: sessionID,
			Msg:       string(sysMsg),
			CreatedAt: now,
		}); err != nil {
			return "", fmt.Errorf("保存系统消息失败: %v", err)
		}
	} else {
		_, ok, err := m.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID)
		if err != nil || !ok {
			return "", fmt.Errorf("会话不存在: %s", sessionID)
		}
		_ = m.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_RUNNING)
	}

	return sessionID, nil
}

// ValidateAndPrepareResume 校验 Resume 请求合法性，更新中断记录状态，并构造放行与拒绝工具集合
func (m *SessionManager) ValidateAndPrepareResume(ctx context.Context, req *pb.ResumeRequest) (map[string]bool, map[string]string, error) {
	_, user := common.UserFromContext(ctx)
	sessionID := req.SessionId
	interruptID := req.InterruptId

	if sessionID == "" || interruptID == "" {
		return nil, nil, fmt.Errorf("session_id 与 interrupt_id 不能为空")
	}

	sess, ok, err := m.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID)
	if err != nil || !ok {
		return nil, nil, fmt.Errorf("会话不存在")
	}
	if sess.Status != pb.SessionStatus_SS_INTERRUPTED {
		return nil, nil, fmt.Errorf("会话当前不处于中断挂起状态")
	}

	interrupt, ok, err := m.allDb.Base.NocliInterruptRepo.GetByInterruptID(ctx, interruptID)
	if err != nil || !ok {
		return nil, nil, fmt.Errorf("中断记录不存在")
	}
	if interrupt.Status != pb.InterruptStatus_IS_PENDING {
		return nil, nil, fmt.Errorf("该中断已被处理或已失效")
	}

	now := time.Now().Unix()
	approvedTools := m.LoadSessionApprovedTools(ctx, sessionID)
	rejectedTools := make(map[string]string)

	if req.Action == pb.ResumeAction_RA_APPROVE {
		approvedTools[interrupt.ToolCallID] = true
		if req.ApproveScope == pb.ApproveScope_AS_SESSION_TOOL {
			approvedTools[interrupt.ToolName] = true
		}
		_ = m.allDb.Base.NocliInterruptRepo.UpdateStatus(ctx, interruptID, pb.InterruptStatus_IS_APPROVED, req.ApproveScope, now, user.Openid, req.Reason)
	} else {
		reason := req.Reason
		if reason == "" {
			reason = "用户拒绝执行该操作"
		}
		rejectedTools[interrupt.ToolCallID] = reason
		_ = m.allDb.Base.NocliInterruptRepo.UpdateStatus(ctx, interruptID, pb.InterruptStatus_IS_REJECTED, req.ApproveScope, now, user.Openid, reason)
	}

	_ = m.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_RUNNING)

	return approvedTools, rejectedTools, nil
}

// FinalizeSessionTurn 统一收尾处理：批量保存新产生的增量消息，并在 IDLE / INTERRUPTED 时更新数据库 SessionStatus
func (m *SessionManager) FinalizeSessionTurn(
	ctx context.Context,
	sessionID string,
	newMsgs []openai.ChatCompletionMessage,
	pendingInterrupt *dataBase.NocliInterruptModel,
	finalStatus pb.SessionStatus,
	checkpointMsg *openai.ChatCompletionMessage,
) error {
	return m.allDb.Base.InTransaction(ctx, func(txCtx context.Context) error {
		now := time.Now().Unix()

		// 1. 如果触发了上下文压缩，先落盘 Checkpoint 消息 (MsgTypeCheckpoint = 1) 并更新 session 表状态
		if checkpointMsg != nil {
			bytes, err := json.Marshal(checkpointMsg)
			if err != nil {
				return fmt.Errorf("序列化 Checkpoint 消息失败: %v", err)
			}
			cpModel := &dataBase.NocliMessageModel{
				SessionID: sessionID,
				MsgType:   dataBase.MsgTypeCheckpoint,
				Msg:       string(bytes),
				CreatedAt: now,
			}
			if err := m.allDb.Base.NocliMessageRepo.Create(txCtx, cpModel); err != nil {
				return fmt.Errorf("保存 Checkpoint 消息失败: %v", err)
			}

			// 查询会话当前 compress_count
			sessModel, ok, err := m.allDb.Base.NocliSessionRepo.GetBySessionID(txCtx, sessionID)
			var currentCount int32 = 0
			if err == nil && ok {
				currentCount = sessModel.CompressCount
			}
			_ = m.allDb.Base.NocliSessionRepo.UpdateCompressStatus(txCtx, sessionID, currentCount+1, cpModel.ID, now)
		}

		// 2. 落盘待挂起的中断事件
		if pendingInterrupt != nil {
			if err := m.allDb.Base.NocliInterruptRepo.CreateBatch(txCtx, []*dataBase.NocliInterruptModel{pendingInterrupt}); err != nil {
				return fmt.Errorf("创建中断记录失败: %v", err)
			}
			_ = m.allDb.Base.NocliSessionRepo.UpdateStatus(txCtx, sessionID, pb.SessionStatus_SS_INTERRUPTED)
		}

		// 3. 批量追加保存产生的增量消息
		if len(newMsgs) > 0 {
			if err := m.SaveHistory(txCtx, sessionID, newMsgs); err != nil {
				return fmt.Errorf("保存对话历史失败: %v", err)
			}
		}

		if finalStatus == pb.SessionStatus_SS_IDLE {
			_ = m.allDb.Base.NocliSessionRepo.UpdateStatus(txCtx, sessionID, pb.SessionStatus_SS_IDLE)
		}

		return nil
	})
}

// ListSessions 分页获取当前用户的历史会话列表
func (m *SessionManager) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	_, user := common.UserFromContext(ctx)
	openid := user.Openid
	if openid == "" {
		openid = "default_user"
	}

	var pageNum int32 = 1
	var pageSize int32 = 20
	if req != nil && req.Page != nil {
		if req.Page.Number > 0 {
			pageNum = int32(req.Page.Number)
		}
		if req.Page.Size > 0 {
			pageSize = int32(req.Page.Size)
		}
	}

	models, total, err := m.allDb.Base.NocliSessionRepo.ListByOpenid(ctx, openid, pageNum, pageSize)
	if err != nil {
		return nil, fmt.Errorf("查询会话列表失败: %v", err)
	}

	sessions := make([]*pb.SessionInfo, 0, len(models))
	for _, item := range models {
		sessions = append(sessions, &pb.SessionInfo{
			SessionId: item.SessionID,
			Name:      item.Name,
			Status:    item.Status,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}

	return &pb.ListSessionsResponse{
		Sessions: sessions,
		Total:    int32(total),
	}, nil
}

// DeleteSession 删除指定会话（包含级联删除关联的消息记录与中断记录，开启事务保障原子性）
func (m *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session_id 不能为空")
	}

	return m.allDb.Base.InTransaction(ctx, func(txCtx context.Context) error {
		if err := m.allDb.Base.NocliSessionRepo.DeleteBySessionID(txCtx, sessionID); err != nil {
			return fmt.Errorf("删除会话主表失败: %v", err)
		}
		if err := m.allDb.Base.NocliMessageRepo.DeleteBySessionID(txCtx, sessionID); err != nil {
			return fmt.Errorf("删除会话消息历史失败: %v", err)
		}
		if err := m.allDb.Base.NocliInterruptRepo.DeleteBySessionID(txCtx, sessionID); err != nil {
			return fmt.Errorf("删除会话中断记录失败: %v", err)
		}
		return nil
	})
}

// MapMessageModelToStreamChunks 将数据库持久化的 NocliMessageModel 映射为前端统一渲染的回放态 StreamChunk 切片
func MapMessageModelToStreamChunks(sessionID string, model dataBase.NocliMessageModel) []*pb.StreamChunk {
	var chatMsg openai.ChatCompletionMessage
	if err := json.Unmarshal([]byte(model.Msg), &chatMsg); err != nil {
		return nil
	}

	chunks := make([]*pb.StreamChunk, 0)

	// 1. 如果是 Checkpoint 描述消息 (MsgType == 1)，精准映射为 SET_CONTEXT_COMPRESSED 弹卡
	if model.MsgType == dataBase.MsgTypeCheckpoint {
		chunks = append(chunks, &pb.StreamChunk{
			Event:     pb.StreamEventType_SET_CONTEXT_COMPRESSED,
			Role:      "system",
			SessionId: sessionID,
			Text:      chatMsg.Content,
			CompressInfo: &pb.CompressInfo{
				SummaryPreview: chatMsg.Content,
				Status:         pb.CompressStatus_CS_COMPLETED,
			},
		})
		return chunks
	}

	// 2. 如果是普通 System 消息 (如 Agent 角色设定提示词)，不在前端聊天流中重复渲染为卡片
	if chatMsg.Role == "system" {
		return nil
	}

	switch chatMsg.Role {
	case openai.ChatMessageRoleUser:
		chunks = append(chunks, &pb.StreamChunk{
			Event:     pb.StreamEventType_SET_DONE,
			Role:      chatMsg.Role,
			SessionId: sessionID,
			Status:    pb.SessionStatus_SS_IDLE,
			Text:      chatMsg.Content,
		})

	case openai.ChatMessageRoleAssistant:
		if chatMsg.Content != "" || chatMsg.ReasoningContent != "" {
			chunks = append(chunks, &pb.StreamChunk{
				Event:         pb.StreamEventType_SET_TEXT_DELTA,
				Role:          chatMsg.Role,
				SessionId:     sessionID,
				Text:          chatMsg.Content,
				ReasoningText: chatMsg.ReasoningContent,
			})
		}
		for _, tc := range chatMsg.ToolCalls {
			chunks = append(chunks, &pb.StreamChunk{
				Event:     pb.StreamEventType_SET_TOOL_START,
				Role:      chatMsg.Role,
				SessionId: sessionID,
				ToolInfo: &pb.StreamToolInfo{
					ToolCallId: tc.ID,
					ToolName:   tc.Function.Name,
					Arguments:  tc.Function.Arguments,
				},
			})
		}

	case openai.ChatMessageRoleTool:
		chunks = append(chunks, &pb.StreamChunk{
			Event:     pb.StreamEventType_SET_TOOL_RESULT,
			Role:      chatMsg.Role,
			SessionId: sessionID,
			ToolInfo: &pb.StreamToolInfo{
				ToolCallId:    chatMsg.ToolCallID,
				ResultPreview: TruncateText(chatMsg.Content, 200),
			},
		})
	}

	return chunks
}

// GetSessionHistory 获取指定会话的历史记录与挂起的中断信息 (方案 A: 返回用于前端统一渲染与回放的 StreamChunk 切片)
func (m *SessionManager) GetSessionHistory(ctx context.Context, req *pb.GetSessionHistoryRequest) (*pb.GetSessionHistoryResponse, error) {
	if req == nil || req.SessionId == "" {
		return nil, fmt.Errorf("session_id 不能为空")
	}
	sessionID := req.SessionId
	var pageNum int32 = 1
	var pageSize int32 = 20
	if req.Page != nil {
		if req.Page.Number > 0 {
			pageNum = int32(req.Page.Number)
		}
		if req.Page.Size > 0 {
			pageSize = int32(req.Page.Size)
		}
	}

	// 1. 查询会话状态
	sessionModel, found, err := m.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("获取会话信息失败: %v", err)
	}
	if !found {
		return nil, fmt.Errorf("会话 %s 不存在", sessionID)
	}

	// 2. 分页加载会话消息历史并映射为 StreamChunk 回放包
	msgModels, total, err := m.allDb.Base.NocliMessageRepo.ListBySessionIDPage(ctx, sessionID, pageNum, pageSize)
	if err != nil {
		return nil, fmt.Errorf("获取会话消息历史失败: %v", err)
	}

	chunks := make([]*pb.StreamChunk, 0, len(msgModels)*2)
	for _, mItem := range msgModels {
		if itemChunks := MapMessageModelToStreamChunks(sessionID, mItem); len(itemChunks) > 0 {
			chunks = append(chunks, itemChunks...)
		}
	}

	hasMore := int64((pageNum-1)*pageSize+int32(len(msgModels))) < total

	// 3. 检查是否有挂起的中断调用并加载
	var pendingCalls []*pb.PendingToolCall
	interrupts, err := m.allDb.Base.NocliInterruptRepo.GetPendingBySessionID(ctx, sessionID)
	if err == nil && len(interrupts) > 0 {
		for _, item := range interrupts {
			pendingCalls = append(pendingCalls, &pb.PendingToolCall{
				InterruptId: item.InterruptID,
				ToolCallId:  item.ToolCallID,
				ToolName:    item.ToolName,
				Arguments:   item.Arguments,
			})
		}
		sessionModel.Status = pb.SessionStatus_SS_INTERRUPTED
	}

	return &pb.GetSessionHistoryResponse{
		SessionId:        sessionID,
		Status:           sessionModel.Status,
		Chunks:           chunks,
		PendingToolCalls: pendingCalls,
		Total:            int32(total),
		HasMore:          hasMore,
	}, nil
}
