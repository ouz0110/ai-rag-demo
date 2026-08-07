package observability

import (
	"context"
	"strings"

	"github.com/google/uuid"
	openai "github.com/sashabaranov/go-openai"
)

type ctxKey string

const (
	observerKey  ctxKey = "obs_instance"
	requestIDKey ctxKey = "obs_request_id"
	sessionIDKey ctxKey = "obs_session_id"
	agentNameKey ctxKey = "obs_agent_name"
)

// WithObserver 将 Observer 实例塞入 Context
func WithObserver(ctx context.Context, obs Observer) context.Context {
	if obs == nil {
		return ctx
	}
	return context.WithValue(ctx, observerKey, obs)
}

// GetObserver 从 Context 提取 Observer 实例，若未注入则返回 NopObserver
func GetObserver(ctx context.Context) Observer {
	if obs, ok := ctx.Value(observerKey).(Observer); ok && obs != nil {
		return obs
	}
	return NopObserver{}
}

// WithRequestID 将 RequestID 绑定到 Context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		requestID = "req-" + uuid.New().String()
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID 从 Context 提取 RequestID
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok && id != "" {
		return id
	}
	return ""
}

// WithSessionID 将 SessionID 绑定到 Context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	if sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// GetSessionID 从 Context 提取 SessionID
func GetSessionID(ctx context.Context) string {
	if id, ok := ctx.Value(sessionIDKey).(string); ok && id != "" {
		return id
	}
	return ""
}

// WithAgentName 将 AgentName 绑定到 Context
func WithAgentName(ctx context.Context, agentName string) context.Context {
	if agentName == "" {
		return ctx
	}
	return context.WithValue(ctx, agentNameKey, agentName)
}

// GetAgentName 从 Context 提取 AgentName
func GetAgentName(ctx context.Context) string {
	if name, ok := ctx.Value(agentNameKey).(string); ok && name != "" {
		return name
	}
	return ""
}

// TruncateSummary 截取文本摘要 (默认截取前 100 字符，超长以 ... 结尾，单行化处理)
func TruncateSummary(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 100
	}
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	runes := []rune(s)
	if len(runes) <= maxLen {
		return string(runes)
	}
	return string(runes[:maxLen]) + "..."
}

// NopObserver 空观测者 (默认保底实现，避免空指针)
type NopObserver struct{}

func (NopObserver) OnAgentStart(ctx context.Context, info *AgentRunInfo) (context.Context, EndAgentFunc) {
	return ctx, func(reply string, err error) {}
}

func (NopObserver) OnLLMStart(ctx context.Context, info *LLMCallInfo) (context.Context, EndLLMFunc) {
	return ctx, func(msg *openai.ChatCompletionMessage, err error) {}
}

func (NopObserver) OnToolStart(ctx context.Context, info *ToolCallInfo) (context.Context, EndToolFunc) {
	return ctx, func(result string, err error) {}
}

func (NopObserver) OnCompressStart(ctx context.Context, info *CompressInfo) (context.Context, EndCompressFunc) {
	return ctx, func(compressedTokens int, isMaxLimit bool, summaryText string, err error) {}
}
