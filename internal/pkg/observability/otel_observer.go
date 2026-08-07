package observability

import (
	"context"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var defaultTracer = otel.Tracer("ai-rag-demo/llm")

// OTelObserver OpenTelemetry 全链路 Tracing 观测者实现
// 遵守 OpenTelemetry GenAI 国际语义约定 (Semantic Conventions)，生成标准的树状 Span
type OTelObserver struct {
	tracer trace.Tracer
}

func NewOTelObserver(customTracer trace.Tracer) *OTelObserver {
	if customTracer == nil {
		customTracer = defaultTracer
	}
	return &OTelObserver{tracer: customTracer}
}

func (o *OTelObserver) OnAgentStart(ctx context.Context, info *AgentRunInfo) (context.Context, EndAgentFunc) {
	ensureMetadata(ctx, &info.SessionID, &info.AgentName)
	requestID := GetRequestID(ctx)
	newCtx, span := o.tracer.Start(ctx, "agent.run",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("request_id", requestID),
			attribute.String("session_id", info.SessionID),
			attribute.String("gen_ai.agent.name", info.AgentName),
			attribute.String("gen_ai.request.model", info.Model),
			attribute.Int("gen_ai.agent.max_iterations", info.MaxIterations),
			attribute.String("gen_ai.agent.timeout", info.Timeout.String()),
		),
	)

	start := time.Now()
	return newCtx, func(reply string, err error) {
		duration := time.Since(start)
		span.SetAttributes(attribute.Int64("gen_ai.duration_ms", duration.Milliseconds()))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("gen_ai.agent.reply_bytes", len(reply)))
			span.SetStatus(codes.Ok, "success")
		}
		span.End()
	}
}

func (o *OTelObserver) OnLLMStart(ctx context.Context, info *LLMCallInfo) (context.Context, EndLLMFunc) {
	ensureMetadata(ctx, &info.SessionID, &info.AgentName)
	requestID := GetRequestID(ctx)
	newCtx, span := o.tracer.Start(ctx, "gen_ai.chat_model.generate",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("request_id", requestID),
			attribute.String("session_id", info.SessionID),
			attribute.String("gen_ai.agent.name", info.AgentName),
			attribute.String("gen_ai.system", "openai"),
			attribute.String("gen_ai.request.model", info.Model),
			attribute.Int("gen_ai.request.iteration", info.Iteration),
			attribute.Int("gen_ai.request.messages_count", info.MessagesCount),
			attribute.Int("gen_ai.request.tools_count", info.ToolsCount),
		),
	)

	start := time.Now()
	return newCtx, func(msg *openai.ChatCompletionMessage, err error) {
		duration := time.Since(start)
		span.SetAttributes(attribute.Int64("gen_ai.duration_ms", duration.Milliseconds()))

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else if msg != nil {
			span.SetAttributes(
				attribute.String("gen_ai.completion.role", string(msg.Role)),
				attribute.Int("gen_ai.completion.content_bytes", len(msg.Content)),
				attribute.Int("gen_ai.completion.tool_calls_count", len(msg.ToolCalls)),
			)
			span.SetStatus(codes.Ok, "success")
		}
		span.End()
	}
}

func (o *OTelObserver) OnToolStart(ctx context.Context, info *ToolCallInfo) (context.Context, EndToolFunc) {
	ensureMetadata(ctx, &info.SessionID, &info.AgentName)
	requestID := GetRequestID(ctx)
	newCtx, span := o.tracer.Start(ctx, "gen_ai.tool.call",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("request_id", requestID),
			attribute.String("session_id", info.SessionID),
			attribute.String("gen_ai.agent.name", info.AgentName),
			attribute.String("gen_ai.tool.name", info.ToolName),
			attribute.Int("gen_ai.tool.args_bytes", len(info.ArgsJSON)),
		),
	)

	start := time.Now()
	return newCtx, func(result string, err error) {
		duration := time.Since(start)
		span.SetAttributes(attribute.Int64("gen_ai.tool.duration_ms", duration.Milliseconds()))

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("gen_ai.tool.result_bytes", len(result)))
			span.SetStatus(codes.Ok, "success")
		}
		span.End()
	}
}

func (o *OTelObserver) OnCompressStart(ctx context.Context, info *CompressInfo) (context.Context, EndCompressFunc) {
	ensureMetadata(ctx, &info.SessionID, &info.AgentName)
	requestID := GetRequestID(ctx)
	newCtx, span := o.tracer.Start(ctx, "gen_ai.context_compressor",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("request_id", requestID),
			attribute.String("session_id", info.SessionID),
			attribute.String("gen_ai.agent.name", info.AgentName),
			attribute.Int("gen_ai.compress.original_tokens", info.OriginalTokens),
			attribute.Int("gen_ai.compress.compress_count", int(info.CompressCount)),
		),
	)

	start := time.Now()
	return newCtx, func(compressedTokens int, isMaxLimit bool, summaryText string, err error) {
		duration := time.Since(start)
		span.SetAttributes(attribute.Int64("gen_ai.compress.duration_ms", duration.Milliseconds()))

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			savedTokens := info.OriginalTokens - compressedTokens
			span.SetAttributes(
				attribute.Int("gen_ai.compress.compressed_tokens", compressedTokens),
				attribute.Int("gen_ai.compress.saved_tokens", savedTokens),
				attribute.Bool("gen_ai.compress.is_max_limit", isMaxLimit),
				attribute.String("gen_ai.compress.summary_snippet", TruncateSummary(summaryText, 100)),
			)
			span.SetStatus(codes.Ok, "success")
		}
		span.End()
	}
}

// Ensure interface compliance
var _ Observer = (*OTelObserver)(nil)
