package nocli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	codeanalyzer "ai-rag-demo/internal/biz/nocli/openai/prompt/code_analyzer"
	"ai-rag-demo/internal/common"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/utils"

	openai "github.com/sashabaranov/go-openai"
)

// initOrCreateSession 初始化或加载会话，负责新建会话与 SystemPrompt 的安全落盘
func (s *ChatBiz) initOrCreateSession(ctx context.Context, sessionID, userMsg string) (string, error) {
	workDir := s.cfg.Source.Nocli.WorkDir
	if workDir == "" {
		workDir = "."
	}
	systemPrompt := codeanalyzer.SystemPrompt(workDir)
	_, user := common.UserFromContext(ctx)
	now := time.Now().Unix()

	if sessionID == "" {
		sessionID = utils.NewUUID()
		session := &dataBase.NocliSessionModel{
			SessionID: sessionID,
			Openid:    user.Openid,
			Name:      truncateText(userMsg, 30),
			Status:    pb.SessionStatus_SS_RUNNING,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.allDb.Base.NocliSessionRepo.Create(ctx, session); err != nil {
			return "", fmt.Errorf("创建会话失败: %v", err)
		}
		log.Debugw(ctx, "session_created", "session_id", sessionID)

		sysMsg, _ := json.Marshal(openai.ChatCompletionMessage{
			Role: "system", Content: systemPrompt,
		})
		if err := s.allDb.Base.NocliMessageRepo.Create(ctx, &dataBase.NocliMessageModel{
			SessionID: sessionID,
			Msg:       string(sysMsg),
			CreatedAt: now,
		}); err != nil {
			return "", fmt.Errorf("保存系统消息失败: %v", err)
		}
	} else {
		_, ok, err := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID)
		if err != nil || !ok {
			return "", fmt.Errorf("会话不存在")
		}

		_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_RUNNING)
		log.Debugw(ctx, "session_loaded", "session_id", sessionID)
	}

	return sessionID, nil
}

// prepareMessagesForCompletion 载入历史消息、自动处理旧未决中断，并组装 User 消息
func (s *ChatBiz) prepareMessagesForCompletion(ctx context.Context, sessionID, userMsg string) ([]openai.ChatCompletionMessage, int, error) {
	messages, err := s.loadHistory(ctx, sessionID)
	if err != nil {
		return nil, 0, fmt.Errorf("加载对话历史失败: %v", err)
	}

	newMessageStart := len(messages)

	// 若之前有未处理中断，当用户直接发新提问时，自动作废旧中断并将取消消息 append 到内存消息数组中
	cancelMsgs, _ := s.cancelPendingInterruptsOnNewCompletion(ctx, sessionID)
	if len(cancelMsgs) > 0 {
		messages = append(messages, cancelMsgs...)
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role: "user", Content: userMsg,
	})

	return messages, newMessageStart, nil
}

// validateAndPrepareResume 校验 Resume 请求合法性，更新中断记录状态，并构造放行与拒绝工具集合
func (s *ChatBiz) validateAndPrepareResume(ctx context.Context, req *pb.ResumeRequest) (map[string]bool, map[string]string, error) {
	_, user := common.UserFromContext(ctx)
	sessionID := req.SessionId
	interruptID := req.InterruptId

	if sessionID == "" || interruptID == "" {
		return nil, nil, fmt.Errorf("session_id 与 interrupt_id 不能为空")
	}

	session, ok, err := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID)
	if err != nil || !ok {
		return nil, nil, fmt.Errorf("会话不存在")
	}
	if session.Status != pb.SessionStatus_SS_INTERRUPTED {
		return nil, nil, fmt.Errorf("会话当前不处于中断挂起状态")
	}

	interrupt, ok, err := s.allDb.Base.NocliInterruptRepo.GetByInterruptID(ctx, interruptID)
	if err != nil || !ok {
		return nil, nil, fmt.Errorf("中断记录不存在")
	}
	if interrupt.Status != pb.InterruptStatus_IS_PENDING {
		return nil, nil, fmt.Errorf("该中断已被处理或已失效")
	}

	now := time.Now().Unix()
	approvedTools := make(map[string]bool)
	rejectedTools := make(map[string]string)

	if req.Action == pb.ResumeAction_RA_APPROVE {
		approvedTools[interrupt.ToolCallID] = true
		if req.ApproveScope == pb.ApproveScope_AS_SESSION_TOOL {
			approvedTools[interrupt.ToolName] = true
		}
		_ = s.allDb.Base.NocliInterruptRepo.UpdateStatus(ctx, interruptID, pb.InterruptStatus_IS_APPROVED, now, user.Openid, req.Reason)
	} else {
		reason := req.Reason
		if reason == "" {
			reason = "用户拒绝执行该操作"
		}
		rejectedTools[interrupt.ToolCallID] = reason
		_ = s.allDb.Base.NocliInterruptRepo.UpdateStatus(ctx, interruptID, pb.InterruptStatus_IS_REJECTED, now, user.Openid, reason)
	}

	_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_RUNNING)

	return approvedTools, rejectedTools, nil
}

// finalizeSessionTurn 统一收尾处理：批量保存新产生的增量消息，并在 IDLE 时更新数据库 SessionStatus
func (s *ChatBiz) finalizeSessionTurn(ctx context.Context, sessionID string, newMsgs []openai.ChatCompletionMessage, finalStatus pb.SessionStatus) error {
	if len(newMsgs) > 0 {
		if err := s.saveHistory(ctx, sessionID, newMsgs); err != nil {
			return fmt.Errorf("保存对话历史失败: %v", err)
		}
	}

	if finalStatus == pb.SessionStatus_SS_IDLE {
		_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_IDLE)
	}

	return nil
}
