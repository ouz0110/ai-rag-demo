package base

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
)

const (
	ParentSessionIDKey     = "parent_session_id"
	ParentMessagesKey      = "parent_messages"
	ParentAppenderKey      = "parent_messages_appender"
	ParentEmitterKey       = "parent_emitter"
	ParentKBTenantIDKey    = "parent_kb_tenant_id"
	ParentKBIDKey          = "parent_kb_id"
	ParentEnableRAGKey     = "parent_enable_rag"
	ParentKBNameKey        = "parent_kb_name"
	ParentKBDescriptionKey = "parent_kb_description"
)

type ParentContext struct {
	SessionID     string
	KBTenantID    string
	KBID          string
	KBName        string
	KBDescription string
	EnableRAG     bool
	Messages      []openai.ChatCompletionMessage
	Appender      func([]openai.ChatCompletionMessage)
	Emitter       StreamEmitter
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
	if pc.KBName != "" {
		ctx = context.WithValue(ctx, ParentKBNameKey, pc.KBName)
	}
	if pc.KBDescription != "" {
		ctx = context.WithValue(ctx, ParentKBDescriptionKey, pc.KBDescription)
	}
	if pc.Emitter != nil {
		ctx = context.WithValue(ctx, ParentEmitterKey, pc.Emitter)
	}
	return ctx
}
