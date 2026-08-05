package base

import (
	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli/checkpoint"
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
	ParentEnableSkillKey   = "parent_enable_skill"
	ParentEnableMCPKey     = "parent_enable_mcp"
	ParentEnableRerankKey      = "parent_enable_rerank"
	ParentAgentToolOptionsKey = "parent_agent_tool_options"
	ParentSubMsgBufferKey     = "parent_sub_msg_buffer"
	ParentPendingToolCallKey  = "parent_pending_tool_call"
	ParentToolCallIDKey       = "parent_tool_call_id"
	ParentToolDurationsKey    = "parent_tool_durations"
	ParentKBNameKey            = "parent_kb_name"
	ParentKBDescriptionKey     = "parent_kb_description"
)

type AgentToolOptions = checkpoint.AgentToolOptions

type ParentContext struct {
	SessionID        string
	KBTenantID       string
	KBID             string
	KBName           string
	KBDescription    string
	EnableRAG        bool
	EnableSkill      bool
	EnableMCP        bool
	EnableRerank     bool
	AgentToolOptions AgentToolOptions
	Messages         []openai.ChatCompletionMessage
	SubMsgBuffer     *[]openai.ChatCompletionMessage
	PendingToolCall  **pb.PendingToolCall
	Appender         func([]openai.ChatCompletionMessage)
	Emitter          StreamEmitter
}

func (pc *ParentContext) Inject(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, ParentSessionIDKey, pc.SessionID)
	ctx = context.WithValue(ctx, ParentMessagesKey, pc.Messages)
	ctx = context.WithValue(ctx, ParentAppenderKey, pc.Appender)
	if pc.SubMsgBuffer != nil {
		ctx = context.WithValue(ctx, ParentSubMsgBufferKey, pc.SubMsgBuffer)
	}
	if pc.PendingToolCall != nil {
		ctx = context.WithValue(ctx, ParentPendingToolCallKey, pc.PendingToolCall)
	}
	ctx = context.WithValue(ctx, ParentEnableRAGKey, pc.EnableRAG)
	ctx = context.WithValue(ctx, ParentEnableSkillKey, pc.EnableSkill)
	ctx = context.WithValue(ctx, ParentEnableMCPKey, pc.EnableMCP)
	ctx = context.WithValue(ctx, ParentEnableRerankKey, pc.EnableRerank)
	ctx = context.WithValue(ctx, ParentAgentToolOptionsKey, pc.AgentToolOptions)
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
