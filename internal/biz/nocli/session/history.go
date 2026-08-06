package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	agentbase "ai-rag-demo/internal/biz/nocli/openai/agent/base"
	"ai-rag-demo/internal/common"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

// StoredChatMessage 封装包含耗时等扩展元数据的落盘消息数据结构 (兼容量纲与重放还原)
type StoredChatMessage struct {
	openai.ChatCompletionMessage
	DurationMs int64 `json:"duration_ms,omitempty"`
}

// LoadHistory 加载该会话在数据库中的完整历史消息集合 (客户端全量呈现使用)
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

// LoadHistoryForLLM 为 LLM 运行时精准加载【首部初始 System + 最新 Checkpoint 摘要 + 增量消息】，并排除子 Agent 内部细节消息
func (m *SessionManager) LoadHistoryForLLM(ctx context.Context, sessionID string) ([]openai.ChatCompletionMessage, error) {
	sess, ok, err := m.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID)
	if err != nil || !ok {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	var rawMsgs []openai.ChatCompletionMessage

	// 1. 若未发生过 Checkpoint 压缩，直接加载全量历史
	if sess.LastCheckpointMsgID == 0 {
		var err error
		rawMsgs, err = m.LoadHistory(ctx, sessionID)
		if err != nil {
			return nil, err
		}
	} else {
		// 2. 加载首部初始 System 消息群 (开头连续的 system 消息)
		allModels, err := m.allDb.Base.NocliMessageRepo.GetBySessionID(ctx, sessionID)
		if err != nil {
			return nil, err
		}

		initialSysMsgs := make([]openai.ChatCompletionMessage, 0)
		for _, model := range allModels {
			var msg openai.ChatCompletionMessage
			if err := json.Unmarshal([]byte(model.Msg), &msg); err != nil {
				continue
			}
			if msg.Role == openai.ChatMessageRoleSystem {
				initialSysMsgs = append(initialSysMsgs, msg)
			} else {
				break
			}
		}

		// 3. 从 ID >= LastCheckpointMsgID 检索最新 Checkpoint 及其后续增量消息
		incModels, err := m.allDb.Base.NocliMessageRepo.GetMessagesFromID(ctx, sessionID, sess.LastCheckpointMsgID)
		if err != nil {
			return nil, err
		}

		rawMsgs = make([]openai.ChatCompletionMessage, 0, len(initialSysMsgs)+len(incModels))
		rawMsgs = append(rawMsgs, initialSysMsgs...)

		for _, model := range incModels {
			var msg openai.ChatCompletionMessage
			if err := json.Unmarshal([]byte(model.Msg), &msg); err != nil {
				continue
			}
			rawMsgs = append(rawMsgs, msg)
		}
	}

	// 🎯 关键步骤：过滤子 Agent 内部细节消息 (仅留 Web 端回放展示，主 Agent LLM 运行时完全剔除)
	return FilterSubAgentMessagesForLLM(rawMsgs), nil
}

// FilterSubAgentMessagesForLLM 过滤掉为 Web 展示保存的子 Agent 内部多轮细节消息 (如 read_files/list_files)
// 保证 LLM 运行时接收到的历史上下文极简、无 Token 冲爆隐患，且仅包含主 Agent 的委派指令与总结 ToolResult。
func FilterSubAgentMessagesForLLM(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	return agentbase.FilterSubAgentMessagesForLLM(msgs)
}

// PrepareMessagesForCompletion 为 Completion 准备消息：处理过期中断、追加用户新消息并清洗未决 ToolCalls
func (m *SessionManager) PrepareMessagesForCompletion(ctx context.Context, sessionID, userMsg string) ([]openai.ChatCompletionMessage, int, error) {
	messages, err := m.LoadHistoryForLLM(ctx, sessionID)
	if err != nil {
		return nil, 0, fmt.Errorf("加载 LLM 对话历史失败: %v", err)
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

// SaveHistory 增量批量落盘保存产生的新消息列表 (支持关联耗时数据扩展落盘，保障回放与重放还原)
func (m *SessionManager) SaveHistory(ctx context.Context, sessionID string, msgs []openai.ChatCompletionMessage, toolDurations map[string]int64) error {
	if len(msgs) == 0 {
		return nil
	}

	now := time.Now().Unix()
	models := make([]*dataBase.NocliMessageModel, 0, len(msgs))

	for _, msg := range msgs {
		var bytes []byte
		var err error

		if msg.ToolCallID != "" && toolDurations != nil {
			if dur, ok := toolDurations[msg.ToolCallID]; ok && dur > 0 {
				bytes, err = json.Marshal(StoredChatMessage{
					ChatCompletionMessage: msg,
					DurationMs:            dur,
				})
			}
		}

		if len(bytes) == 0 {
			bytes, err = json.Marshal(msg)
		}

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

// CleanOrCancelPendingInterrupts 当用户放弃审批直接发送新提问时：作废挂起中断、删快照、并为未闭合的 ToolCall 自动插入取消响应
func (m *SessionManager) CleanOrCancelPendingInterrupts(ctx context.Context, sessionID string) error {
	return m.allDb.Base.InTransaction(ctx, func(txCtx context.Context) error {
		// 1. 将所有 IS_PENDING 中断记录置为已作废/取消
		_ = m.allDb.Base.NocliInterruptRepo.CancelPendingBySessionID(txCtx, sessionID)

		// 2. 删除对应的 SubAgentCheckpoint 快照
		m.ClearSubAgentCheckpoint(sessionID)

		// 3. 读取当前消息历史，检查末尾是否有未响应的 ToolCall
		models, err := m.allDb.Base.NocliMessageRepo.GetBySessionID(txCtx, sessionID)
		if err != nil || len(models) == 0 {
			return nil
		}

		// 检查未闭合的 tool_call_id
		cancelToolMsgs := make([]openai.ChatCompletionMessage, 0)
		lastModel := models[len(models)-1]
		var lastMsg openai.ChatCompletionMessage
		if err := json.Unmarshal([]byte(lastModel.Msg), &lastMsg); err == nil {
			if lastMsg.Role == openai.ChatMessageRoleAssistant && len(lastMsg.ToolCalls) > 0 {
				for _, tc := range lastMsg.ToolCalls {
					cancelToolMsgs = append(cancelToolMsgs, openai.ChatCompletionMessage{
						Role:       openai.ChatMessageRoleTool,
						Content:    "【操作已取消】: 用户放弃了该授权审批并提出了新的问题。",
						ToolCallID: tc.ID,
						Name:       lastMsg.Name,
					})
				}
			}
		}

		// 4. 落盘平滑闭合消息链，避免 LLM API 报 400 错
		if len(cancelToolMsgs) > 0 {
			if err := m.SaveHistory(txCtx, sessionID, cancelToolMsgs, nil); err != nil {
				return fmt.Errorf("保存取消工具消息失败: %w", err)
			}
		}

		// 5. 将会话状态重置为 IDLE
		_ = m.allDb.Base.NocliSessionRepo.UpdateStatus(txCtx, sessionID, pb.SessionStatus_SS_IDLE)

		return nil
	})
}

func TruncateText(str string, maxChars int) string {
	runes := []rune(str)
	if len(runes) > maxChars {
		return string(runes[:maxChars]) + "..."
	}
	return str
}
