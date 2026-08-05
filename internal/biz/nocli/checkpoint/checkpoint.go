package checkpoint

import (
	"context"

	pb "ai-rag-demo/api/nocli/v1"

	openai "github.com/sashabaranov/go-openai"
)

// AgentToolOptions 子 Agent 控制选项
type AgentToolOptions struct {
	PassFullContextToSubAgent bool `json:"pass_full_context_to_sub_agent"`
	ReturnFullContextToParent bool `json:"return_full_context_to_parent"`
	StreamSubAgentExecution   bool `json:"stream_sub_agent_execution"`
}

// SubAgentCheckpoint 子 Agent 专属中断恢复 Checkpoint 快照
type SubAgentCheckpoint struct {
	SessionID        string                         `json:"session_id"`
	InterruptID      string                         `json:"interrupt_id"`
	TargetAgentName  string                         `json:"target_agent_name"`
	ParentToolCallID string                         `json:"parent_tool_call_id"`
	SubMessages      []openai.ChatCompletionMessage `json:"sub_messages"`
	PendingToolCall  *pb.PendingToolCall            `json:"pending_tool_call"`
	AgentToolOptions AgentToolOptions               `json:"agent_tool_options"`
	KBTenantID       string                         `json:"kb_tenant_id"`
	KBID             string                         `json:"kb_id"`
	EnableRAG        bool                           `json:"enable_rag"`
	EnableSkill      bool                           `json:"enable_skill"`
	EnableMCP        bool                           `json:"enable_mcp"`
	EnableRerank     bool                           `json:"enable_rerank"`
	CreatedAt        int64                          `json:"created_at"`
}

// ICheckpointStore Checkpoint 存储策略抽象接口 (支持内存、Redis、MySQL、文件等后续拓展)
type ICheckpointStore interface {
	Save(ctx context.Context, cp *SubAgentCheckpoint) error
	Get(ctx context.Context, sessionID string) (*SubAgentCheckpoint, bool, error)
	Delete(ctx context.Context, sessionID string) error
}
