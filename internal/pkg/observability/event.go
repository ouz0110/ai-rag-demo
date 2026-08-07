package observability

import (
	"time"
)

// AgentRunInfo 封装 Agent 循环启动元数据
type AgentRunInfo struct {
	AgentName     string        `json:"agent_name"`
	SessionID     string        `json:"session_id"`
	Model         string        `json:"model"`
	MaxIterations int           `json:"max_iterations"`
	Timeout       time.Duration `json:"timeout"`
}

// LLMCallInfo 封装大模型 API 单次调用元数据
type LLMCallInfo struct {
	AgentName     string `json:"agent_name"`
	SessionID     string `json:"session_id"`
	Model         string `json:"model"`
	MessagesCount int    `json:"messages_count"`
	ToolsCount    int    `json:"tools_count"`
	Iteration     int    `json:"iteration"`
}

// ToolCallInfo 封装工具执行启动元数据
type ToolCallInfo struct {
	AgentName string `json:"agent_name"`
	SessionID string `json:"session_id"`
	ToolName  string `json:"tool_name"`
	ArgsJSON  string `json:"args_json"`
}

// CompressInfo 封装上下文压缩触发元数据
type CompressInfo struct {
	AgentName      string `json:"agent_name"`
	SessionID      string `json:"session_id"`
	OriginalTokens int    `json:"original_tokens"`
	CompressCount  int32  `json:"compress_count"`
}
