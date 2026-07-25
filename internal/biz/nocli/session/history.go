package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/common"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

// LoadHistory 加载该会话在数据库中的完整历史消息集合
func (m *SessionManager) LoadHistory(ctx context.Context, sessionID string) ([]openai.ChatCompletionMessage, error) {
	models, err := m.allDb.Base.NocliMessageRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	messages := make([]openai.ChatCompletionMessage, 0, len(models))
	for _, model := range models {
		var msg openai.ChatCompletionMessage
		if err := json.Unmarshal([]byte(model.Msg), &msg); err != nil {
			log.Warnw(ctx, "unmarshal_history_msg_error", "msg_id", model.ID, "error", err)
			continue
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// PrepareMessagesForCompletion 为 Completion 准备消息：处理过期中断、追加用户新消息并清洗未决 ToolCalls
func (m *SessionManager) PrepareMessagesForCompletion(ctx context.Context, sessionID, userMsg string) ([]openai.ChatCompletionMessage, int, error) {
	messages, err := m.LoadHistory(ctx, sessionID)
	if err != nil {
		return nil, 0, fmt.Errorf("加载对话历史失败: %v", err)
	}

	cancelMsgs, err := m.CancelPendingInterrupts(ctx, sessionID)
	if err != nil {
		return nil, 0, fmt.Errorf("清理旧中断记录失败: %v", err)
	}
	if len(cancelMsgs) > 0 {
		messages = append(messages, cancelMsgs...)
	}

	newMessageStart := len(messages)
	userMsgStruct := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userMsg,
	}
	messages = append(messages, userMsgStruct)

	return messages, newMessageStart, nil
}

// CancelPendingInterrupts 处理新提问时发生的旧中断清理：将待确认的中断过期，并添加对应的 Tool 拒绝取消消息
func (m *SessionManager) CancelPendingInterrupts(ctx context.Context, sessionID string) ([]openai.ChatCompletionMessage, error) {
	pendingModels, err := m.allDb.Base.NocliInterruptRepo.GetPendingBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if len(pendingModels) == 0 {
		return nil, nil
	}

	_, user := common.UserFromContext(ctx)
	now := time.Now().Unix()
	cancelMsgs := make([]openai.ChatCompletionMessage, 0, len(pendingModels))

	for _, pm := range pendingModels {
		_ = m.allDb.Base.NocliInterruptRepo.UpdateStatus(
			ctx,
			pm.InterruptID,
			pb.InterruptStatus_IS_EXPIRED,
			pm.ApproveScope,
			now,
			user.Openid,
			"用户发送了新问题，自动取消旧中断",
		)
		cancelMsgs = append(cancelMsgs, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    "操作已取消：用户提交了新的提问，放弃了此授权操作",
			ToolCallID: pm.ToolCallID,
		})
	}

	return cancelMsgs, nil
}

// SaveHistory 增量批量落盘保存产生的新消息列表
func (m *SessionManager) SaveHistory(ctx context.Context, sessionID string, msgs []openai.ChatCompletionMessage) error {
	if len(msgs) == 0 {
		return nil
	}

	now := time.Now().Unix()
	models := make([]*dataBase.NocliMessageModel, 0, len(msgs))

	for _, msg := range msgs {
		bytes, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("序列化消息失败: %v", err)
		}
		models = append(models, &dataBase.NocliMessageModel{
			SessionID: sessionID,
			Msg:       string(bytes),
			CreatedAt: now,
		})
	}

	return m.allDb.Base.NocliMessageRepo.CreateBatch(ctx, models)
}

func TruncateText(str string, maxChars int) string {
	runes := []rune(str)
	if len(runes) > maxChars {
		return string(runes[:maxChars]) + "..."
	}
	return str
}
