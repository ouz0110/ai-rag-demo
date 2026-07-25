package nocli

import (
	dataBase "ai-rag-demo/internal/data/base"
	"context"
	"encoding/json"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func (s *ChatBiz) loadHistory(ctx context.Context, sessionID string) ([]openai.ChatCompletionMessage, error) {
	models, err := s.allDb.Base.NocliMessageRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	messages := make([]openai.ChatCompletionMessage, 0, len(models))
	for _, m := range models {
		var msg openai.ChatCompletionMessage
		if err := json.Unmarshal([]byte(m.Msg), &msg); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	// 容错修复：确保历史消息中悬空的 tool_calls 一定有对应的 tool 响应，防止调用 OpenAI 报 400 错
	messages = sanitizeUnclosedToolCalls(messages)

	return messages, nil
}

func sanitizeUnclosedToolCalls(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}

	respondedToolIDs := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleTool && msg.ToolCallID != "" {
			respondedToolIDs[msg.ToolCallID] = true
		}
	}

	sanitized := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, msg := range messages {
		sanitized = append(sanitized, msg)
		if msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				if !respondedToolIDs[tc.ID] {
					sanitized = append(sanitized, openai.ChatCompletionMessage{
						Role:       openai.ChatMessageRoleTool,
						Content:    "操作已取消：未获得用户明确授权响应",
						ToolCallID: tc.ID,
					})
					respondedToolIDs[tc.ID] = true
				}
			}
		}
	}

	return sanitized
}

func (s *ChatBiz) saveHistory(ctx context.Context, sessionID string, messages []openai.ChatCompletionMessage) error {
	if len(messages) == 0 {
		return nil
	}

	_ = s.allDb.Base.NocliSessionRepo.UpdateUpdatedAt(ctx, sessionID, time.Now().Unix())

	now := time.Now().Unix()
	models := make([]*dataBase.NocliMessageModel, 0, len(messages))
	for _, msg := range messages {
		str, err := msg.MarshalJSON()
		if err != nil {
			return err
		}
		models = append(models, &dataBase.NocliMessageModel{
			SessionID: sessionID,
			Msg:       string(str),
			CreatedAt: now,
		})
	}

	return s.allDb.Base.NocliMessageRepo.CreateBatch(ctx, models)
}
