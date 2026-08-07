package observability

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
)

// EndAgentFunc Agent 循环结束回调函数
type EndAgentFunc func(reply string, err error)

// EndLLMFunc LLM API 调用结束回调函数
type EndLLMFunc func(msg *openai.ChatCompletionMessage, err error)

// EndToolFunc 工具调用结束回调函数
type EndToolFunc func(result string, err error)

// EndCompressFunc 历史摘要压缩结束回调函数
type EndCompressFunc func(compressedTokens int, isMaxLimit bool, summaryText string, err error)

// Observer 面向切面的大模型全链路观测者统一抽象接口
// 参照 CloudWeGo Eino / LangChain Callbacks 模式设计，方便插拔支持 OTel、本地日志、Prometheus 或第三方分析平台 (如 Langfuse)
type Observer interface {
	// OnAgentStart Agent 循环执行启动 Hook
	OnAgentStart(ctx context.Context, info *AgentRunInfo) (context.Context, EndAgentFunc)

	// OnLLMStart ChatModel 大模型 API 调用 Hook
	OnLLMStart(ctx context.Context, info *LLMCallInfo) (context.Context, EndLLMFunc)

	// OnToolStart 物理工具 / 子 Agent 工具调用 Hook
	OnToolStart(ctx context.Context, info *ToolCallInfo) (context.Context, EndToolFunc)

	// OnCompressStart ContextCompressor 历史摘要压缩 Hook
	OnCompressStart(ctx context.Context, info *CompressInfo) (context.Context, EndCompressFunc)
}
