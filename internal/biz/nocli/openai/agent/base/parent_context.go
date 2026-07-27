package base

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
)

type ParentContextKey string

const (
	ParentSessionIDKey  ParentContextKey = "parent_session_id"
	ParentMessagesKey   ParentContextKey = "parent_messages"
	ParentAppenderKey   ParentContextKey = "parent_messages_appender"
	ParentEmitterKey    ParentContextKey = "parent_emitter"
	ParentKBTenantIDKey ParentContextKey = "parent_kb_tenant_id"
	ParentKBIDKey       ParentContextKey = "parent_kb_id"
	ParentEnableRAGKey  ParentContextKey = "parent_enable_rag"
)

type ParentContext struct {
	SessionID  string
	KBTenantID string
	KBID       string
	EnableRAG  bool
	Messages   []openai.ChatCompletionMessage
	Appender   func([]openai.ChatCompletionMessage)
	Emitter    StreamEmitter
}

func (pc *ParentContext) Inject(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, ParentSessionIDKey, pc.SessionID)
	ctx = context.WithValue(ctx, ParentMessagesKey, pc.Messages)
	ctx = context.WithValue(ctx, ParentAppenderKey, pc.Appender)
	ctx = context.WithValue(ctx, ParentEnableRAGKey, pc.EnableRAG)
	if pc.KBTenantID != "" {
		ctx = context.WithValue(ctx, ParentKBTenantIDKey, pc.KBTenantID)
	}
	if pc.KBID != "" {
		ctx = context.WithValue(ctx, ParentKBIDKey, pc.KBID)
	}
	if pc.Emitter != nil {
		ctx = context.WithValue(ctx, ParentEmitterKey, pc.Emitter)
	}
	return ctx
}
