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
)

type ParentContext struct {
	SessionID string
	Messages  []openai.ChatCompletionMessage
	Appender  func([]openai.ChatCompletionMessage)
	Emitter   StreamEmitter
}

func (pc *ParentContext) Inject(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, ParentSessionIDKey, pc.SessionID)
	ctx = context.WithValue(ctx, ParentMessagesKey, pc.Messages)
	ctx = context.WithValue(ctx, ParentAppenderKey, pc.Appender)
	if pc.Emitter != nil {
		ctx = context.WithValue(ctx, ParentEmitterKey, pc.Emitter)
	}
	return ctx
}
