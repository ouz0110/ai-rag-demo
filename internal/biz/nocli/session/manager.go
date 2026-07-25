package session

import (
	"context"
	"fmt"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/openai/agent"
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/skill"
	"ai-rag-demo/internal/pkg/utils"

	openai "github.com/sashabaranov/go-openai"
)

type SessionManager struct {
	allDb         *data.DB
	cfg           *conf.Config
	agentRegistry *agent.Registry
	skillManager  *skill.Manager
}

func NewSessionManager(
	allDb *data.DB,
	cfg *conf.Config,
	agentRegistry *agent.Registry,
	skillManager *skill.Manager,
) *SessionManager {
	return &SessionManager{
		allDb:         allDb,
		cfg:           cfg,
		agentRegistry: agentRegistry,
		skillManager:  skillManager,
	}
}

// InitOrCreateSession 初始化或加载会话，负责新建会话与 SystemPrompt 的安全落盘
func (m *SessionManager) InitOrCreateSession(ctx context.Context, sessionID, userMsg string) (string, error) {
	workDir := ""
	if m.cfg != nil && m.cfg.Source.Nocli != nil {
		workDir = m.cfg.Source.Nocli.WorkDir
	}
	if workDir == "" {
		workDir = "."
	}

	ag, ok := m.agentRegistry.Get("main")
	if !ok {
		return "", fmt.Errorf("未找到默认 main agent")
	}
	systemPrompt := ag.SystemPrompt(workDir, m.skillManager)
	_, user := common.UserFromContext(ctx)
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
) error {
	if pendingInterrupt != nil {
		if err := m.allDb.Base.NocliInterruptRepo.CreateBatch(ctx, []*dataBase.NocliInterruptModel{pendingInterrupt}); err != nil {
			return fmt.Errorf("创建中断记录失败: %v", err)
		}
		_ = m.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_INTERRUPTED)
	}

	if len(newMsgs) > 0 {
		if err := m.SaveHistory(ctx, sessionID, newMsgs); err != nil {
			return fmt.Errorf("保存对话历史失败: %v", err)
		}
	}

	if finalStatus == pb.SessionStatus_SS_IDLE {
		_ = m.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_IDLE)
	}

	return nil
}
