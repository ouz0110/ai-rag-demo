package observability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

// LogObserver 本地结构化日志观测者实现
// 格式化输出各节点启动、耗时、异常及 Token 吞吐日志，方便在本地 Console / 日志文件中直接查看与搜排
type LogObserver struct{}

func NewLogObserver() *LogObserver {
	return &LogObserver{}
}

func (l *LogObserver) OnAgentStart(ctx context.Context, info *AgentRunInfo) (context.Context, EndAgentFunc) {
	start := time.Now()
	ensureMetadata(ctx, &info.SessionID, &info.AgentName)
	requestID := GetRequestID(ctx)
	log.Infow(ctx, "obs_agent_start",
		"request_id", requestID,
		"agent_name", info.AgentName,
		"session_id", info.SessionID,
		"model", info.Model,
		"max_iterations", info.MaxIterations,
		"timeout", info.Timeout.String(),
	)

	return ctx, func(reply string, err error) {
		duration := time.Since(start)
		if err != nil {
			log.Errorw(ctx, "obs_agent_end_failed",
				"request_id", requestID,
				"agent_name", info.AgentName,
				"session_id", info.SessionID,
				"duration_ms", duration.Milliseconds(),
				"error", err,
			)
		} else {
			log.Infow(ctx, "obs_agent_end_success",
				"request_id", requestID,
				"agent_name", info.AgentName,
				"session_id", info.SessionID,
				"reply_len", len(reply),
				"reply_summary", TruncateSummary(reply, 100),
				"duration_ms", duration.Milliseconds(),
			)
		}
	}
}

func (l *LogObserver) OnLLMStart(ctx context.Context, info *LLMCallInfo) (context.Context, EndLLMFunc) {
	start := time.Now()
	ensureMetadata(ctx, &info.SessionID, &info.AgentName)
	requestID := GetRequestID(ctx)
	log.Debugw(ctx, "obs_llm_start",
		"request_id", requestID,
		"agent_name", info.AgentName,
		"session_id", info.SessionID,
		"model", info.Model,
		"iteration", info.Iteration,
		"messages_count", info.MessagesCount,
		"tools_count", info.ToolsCount,
	)

	return ctx, func(msg *openai.ChatCompletionMessage, err error) {
		duration := time.Since(start)
		if err != nil {
			log.Warnw(ctx, "obs_llm_end_failed",
				"request_id", requestID,
				"agent_name", info.AgentName,
				"session_id", info.SessionID,
				"iteration", info.Iteration,
				"duration_ms", duration.Milliseconds(),
				"error", err,
			)
		} else if msg != nil {
			var toolSummary string
			if len(msg.ToolCalls) > 0 {
				var calls []string
				for _, tc := range msg.ToolCalls {
					calls = append(calls, fmt.Sprintf("%s(%s)", tc.Function.Name, tc.Function.Arguments))
				}
				toolSummary = strings.Join(calls, "; ")
			}

			log.Infow(ctx, "obs_llm_end_success",
				"request_id", requestID,
				"agent_name", info.AgentName,
				"session_id", info.SessionID,
				"iteration", info.Iteration,
				"role", msg.Role,
				"content_len", len(msg.Content),
				"content_summary", TruncateSummary(msg.Content, 100),
				"tool_calls_count", len(msg.ToolCalls),
				"tool_calls_summary", TruncateSummary(toolSummary, 100),
				"duration_ms", duration.Milliseconds(),
			)
		}
	}
}

func (l *LogObserver) OnToolStart(ctx context.Context, info *ToolCallInfo) (context.Context, EndToolFunc) {
	start := time.Now()
	ensureMetadata(ctx, &info.SessionID, &info.AgentName)
	requestID := GetRequestID(ctx)
	log.Infow(ctx, "obs_tool_start",
		"request_id", requestID,
		"agent_name", info.AgentName,
		"session_id", info.SessionID,
		"tool_name", info.ToolName,
		"args_len", len(info.ArgsJSON),
		"args_summary", TruncateSummary(info.ArgsJSON, 100),
	)

	return ctx, func(result string, err error) {
		duration := time.Since(start)
		if err != nil {
			log.Warnw(ctx, "obs_tool_end_failed",
				"request_id", requestID,
				"agent_name", info.AgentName,
				"session_id", info.SessionID,
				"tool_name", info.ToolName,
				"duration_ms", duration.Milliseconds(),
				"error", err,
			)
		} else {
			log.Infow(ctx, "obs_tool_end_success",
				"request_id", requestID,
				"agent_name", info.AgentName,
				"session_id", info.SessionID,
				"tool_name", info.ToolName,
				"result_len", len(result),
				"result_summary", TruncateSummary(result, 100),
				"duration_ms", duration.Milliseconds(),
			)
		}
	}
}

func (l *LogObserver) OnCompressStart(ctx context.Context, info *CompressInfo) (context.Context, EndCompressFunc) {
	start := time.Now()
	ensureMetadata(ctx, &info.SessionID, &info.AgentName)
	requestID := GetRequestID(ctx)
	log.Infow(ctx, "obs_compress_start",
		"request_id", requestID,
		"agent_name", info.AgentName,
		"session_id", info.SessionID,
		"orig_tokens", info.OriginalTokens,
		"compress_count", info.CompressCount,
	)

	return ctx, func(compressedTokens int, isMaxLimit bool, summaryText string, err error) {
		duration := time.Since(start)
		if err != nil {
			log.Warnw(ctx, "obs_compress_end_failed",
				"request_id", requestID,
				"agent_name", info.AgentName,
				"session_id", info.SessionID,
				"duration_ms", duration.Milliseconds(),
				"error", err,
			)
		} else {
			savedTokens := info.OriginalTokens - compressedTokens
			log.Infow(ctx, "obs_compress_end_success",
				"request_id", requestID,
				"agent_name", info.AgentName,
				"session_id", info.SessionID,
				"orig_tokens", info.OriginalTokens,
				"compressed_tokens", compressedTokens,
				"saved_tokens", savedTokens,
				"is_max_limit", isMaxLimit,
				"summary_snippet", TruncateSummary(summaryText, 100),
				"duration_ms", duration.Milliseconds(),
			)
		}
	}
}

func ensureMetadata(ctx context.Context, sessionID, agentName *string) {
	if sessionID != nil && *sessionID == "" {
		*sessionID = GetSessionID(ctx)
	}
	if agentName != nil && *agentName == "" {
		*agentName = GetAgentName(ctx)
	}
}
