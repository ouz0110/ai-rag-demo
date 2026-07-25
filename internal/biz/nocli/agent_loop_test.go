package nocli

import (
	"errors"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestCleanLLMError(t *testing.T) {
	rawErr := errors.New(`LLM 调用失败: error, status code: 400, status: 400 Bad Request, message: invalid character 'd' looking for beginning of value, body: data:{"error":{"code":"ModelArts.81001","message":"Failed to format request body, ","param":null,"type":"BadRequest"},"error_code":"ModelArts.81001","error_msg":"Failed to format request body, ","span_id":"032d4b232021c2a6aba94b4fe9afd7ac"}`)

	cleaned := cleanLLMError(rawErr)
	if cleaned == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(cleaned.Error(), "ModelArts.81001") || !strings.Contains(cleaned.Error(), "Failed to format request body") {
		t.Errorf("unexpected cleaned error string: %v", cleaned.Error())
	}
}

func TestSanitizeMessages(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{
			Role:      "assistant",
			Content:   "hello",
			ToolCalls: []openai.ToolCall{},
		},
	}

	sanitized := sanitizeMessages(msgs)
	if sanitized[0].ToolCalls != nil {
		t.Errorf("expected nil ToolCalls for empty slice, got non-nil: %v", sanitized[0].ToolCalls)
	}
}
