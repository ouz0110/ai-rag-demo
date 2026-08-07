# 大模型全链路可观测性与分布式追踪设计方案 (LLM Observability & Tracing)

> **版本**: v1.0.0  
> **面向场景**: Kratos + OpenAI Go + ReAct Multi-Agent + ContextCompressor + RAG 检索  
> **参考设计**: OpenTelemetry GenAI Semantic Conventions & CloudWeGo Eino Callback Architecture  

---

## 一、 背景与设计目标

在大模型（LLM）、RAG 检索以及 Multi-Agent 系统的开发与运维中，常规的微服务 APM（仅记录 HTTP/RPC 耗时与 HTTP 状态码）已无法满足观测需求。主要痛点包括：

1. **链路深且多级嵌套**：一个用户请求进入，可能经历 `HTTP Handler` -> `ChatBiz` -> `Main Agent` -> `Sub-Agent` -> `RAG Retriever` (VectorDB + Reranker) -> `ContextCompressor` -> 多次 `ChatModel.Generate` API 调用。
2. **定位耗时瓶颈困难**：响应耗时 10 秒，无法秒级判断是 Milvus 检索慢、Rerank 打分超时、还是 LLM Tool Call 循环了太多轮。
3. **Token 与成本统计缺失**：无法按 `request_id`、`session_id` 或 `kb_tenant_id` 精准统计 Input/Output Token 消耗及费用。

### 本方案核心目标：
- 采用 **OpenTelemetry (`go.opentelemetry.io/otel/trace`)** 标准，实现 LLM 全链路的树状 Span 瀑布图呈现。
- 建立 **RequestID / TraceID / SessionID / TenantID** 的双向透传与关联绑定。
- 借鉴 **CloudWeGo Eino** 的抽象切面思想，设计适配本项目架构的轻量级 Callback / Aspect 机制。
- 遵循 **OpenTelemetry GenAI 语义约定 (Semantic Conventions)**，便于对接 Jaeger、Grafana、Prometheus、Langfuse 或 Phoenix 等主流观测平台。

---

## 二、 借鉴 Eino 的切面抽象设计 (Eino-Inspired Abstraction)

字节跳动 CloudWeGo Eino 框架通过统一的 `callbacks.Handler` 切面（`OnStart` / `OnEnd` / `OnError`）将组件执行与底层可观测性解耦。参考其设计理念，结合本项目的 ReAct Agent 架构，我们抽象出**三级拦截切面**：

```
                      ┌────────────────────────────────────────┐
                      │  TracingAspect (链路追踪统一拦截切面)     │
                      └───────────────────▲────────────────────┘
                                          │
    ┌───────────────────────┬─────────────┴─────────┬──────────────────────┐
    │                       │                       │                      │
┌───┴─────────────┐   ┌─────┴───────────┐   ┌───────┴──────────┐   ┌───────┴──────────┐
│ 1. Agent Aspect │   │ 2. LLM Aspect   │   │ 3. Tool Aspect   │   │4. Compress Aspect│
│ (Agent 迭代/超时) │   │ (ChatModel 调用) │   │ (物理工具/RAG/MCP)│   │ (历史摘要/熔断)   │
└─────────────────┘   └─────────────────┘   └──────────────────┘   └──────────────────┘
```

### 1. 切面 Handler 接口定义

```go
package tracing

import (
	"context"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// AspectHandler LLM 全链路切面回调接口
type AspectHandler interface {
	// Agent 维度 Hook
	OnAgentStart(ctx context.Context, agentName, sessionID string) (context.Context, func(status string, err error))
	
	// ChatModel LLM API 调用 Hook
	OnLLMStart(ctx context.Context, model string, req *openai.ChatCompletionRequest) (context.Context, func(resp *openai.ChatCompletionMessage, err error))
	
	// Tool 工具执行 Hook (包含 Terminal, ReadFiles, RAG, MCP, Sub-Agent)
	OnToolStart(ctx context.Context, toolName string, argsJSON string) (context.Context, func(result string, err error))
	
	// ContextCompressor 历史摘要压缩 Hook
	OnCompressStart(ctx context.Context, origTokens int) (context.Context, func(compressedTokens int, isMaxLimit bool, err error))
}
```

---

## 三、 全局唯一标识与 Context 透传规范

### 1. RequestID 与 TraceID 双向绑定机制

```
 客户端 HTTP / gRPC 请求
    │ 携带 Header: X-Request-ID: req-98f2a1
    ▼
 Kratos Tracing Middleware (顶层 Root Span)
    ├─ 自动生成/提取 TraceID: 4bf92f3577b34da6a3ce929d0e0e4736
    ├─ 绑定 Attribute: request_id = "req-98f2a1"
    ├─ 绑定 Attribute: session_id = "sess-88c201"
    └─ 注入 context.Context
        │
        ├─► 结构化日志 (log.Infow): 自动带出 trace_id & request_id
        └─► OpenTelemetry Trace: 自动向下隐式传递 Parent SpanID
```

### 2. 规范要点：
- **Header 规范**：HTTP Header 统一使用 `X-Request-ID`。如未携带，Kratos 入口中间件自动生成 UUID。
- **协程安全传递**：在启动后台异步任务或并发协程时，必须使用 `common.RunInGoroutine(ctx, func(ctx context.Context) { ... })`，绝不直接使用原生 `go func()`，确保 Context（含 TraceID）不丢失！

---

## 四、 OpenTelemetry 语义约定 (GenAI Semantic Conventions)

项目中所有 Span 属性严格遵循 OpenTelemetry GenAI 国际标准：

| 属性 Key | 类型 | 示例 / 说明 |
| :--- | :--- | :--- |
| `request_id` | `string` | `"req-98f2a1"` (用户请求唯一标识) |
| `session_id` | `string` | `"sess-88c201"` (会话 ID) |
| `kb_tenant_id` | `string` | `"default_tenant"` (知识库租户 ID) |
| `gen_ai.system` | `string` | `"openai"` / `"deepseek"` / `"qianfan"` |
| `gen_ai.request.model` | `string` | `"deepseek-v3.2"` / `"qwen3-embedding-0.6b"` |
| `gen_ai.usage.input_tokens` | `int` | `1250` (输入 Prompt Token 数) |
| `gen_ai.usage.output_tokens` | `int` | `230` (输出 Completion Token 数) |
| `gen_ai.duration_ms` | `int64` | `1450` (节点耗时，单位 ms) |
| `gen_ai.tool.name` | `string` | `"delegate_to_file_analyzer"` / `"terminal"` |
| `gen_ai.agent.name` | `string` | `"main"` / `"file_analyzer"` / `"rag_agent"` |
| `gen_ai.agent.iteration` | `int` | `1` (当前 ReAct 循环轮次) |

---

## 五、 链路 Span 树层级拓扑规范 (Span Hierarchy Topology)

标准一次 Agent 对话在 Jaeger / Grafana 上的瀑布流 Span 树拓扑如下：

```text
ROOT SPAN: HTTP POST /ai-rag-demo.v1.NocliService/StreamChat
 [Attributes: request_id="req-123", session_id="sess-456", peer.address="192.168.1.10"]
 │
 ├── SPAN: agent.run (main)
 │    │ [Attributes: agent.name="main", max_iterations=20, timeout="5m"]
 │    │
 │    ├── SPAN: gen_ai.chat_model.generate (Iteration 1)
 │    │    │ [Attributes: model="deepseek-v3.2", messages_count=4, input_tokens=1500]
 │    │    └─► Response: ToolCall("delegate_to_file_analyzer")
 │    │
 │    ├── SPAN: gen_ai.tool.call (delegate_to_file_analyzer)
 │    │    │ [Attributes: tool.name="delegate_to_file_analyzer"]
 │    │    │
 │    │    └── SPAN: agent.run (file_analyzer)  <-- 子 Agent 独立 Span
 │    │         │ [Attributes: agent.name="file_analyzer", max_iterations=10, timeout="3m"]
 │    │         │
 │    │         ├── SPAN: gen_ai.chat_model.generate (Sub Iteration 1)
 │    │         │    └─► Response: ToolCall("read_files")
 │    │         │
 │    │         ├── SPAN: gen_ai.tool.call (read_files)
 │    │         │    └─► Reading file: main.go (45ms)
 │    │         │
 │    │         └── SPAN: gen_ai.chat_model.generate (Sub Iteration 2)
 │    │              └─► Sub-Agent Summary Output
 │    │
 │    ├── SPAN: gen_ai.context_compressor  <-- 上下文超限触发压缩
 │    │    │ [Attributes: orig_tokens=16384, compressed_tokens=5734, saved_tokens=10650]
 │    │    └─► SPAN: gen_ai.chat_model.generate (Summarizer API Call)
 │    │
 │    └── SPAN: gen_ai.chat_model.generate (Iteration 2)
 │         └─► Final Response to User
```

---

## 六、 分步实施落地指南与代码实现

### 1. Kratos 入口中间件扩展 (`internal/server/middleware/otel.go`)

在中间件中自动绑定 `X-Request-ID` 和全局维度属性：

```go
package middleware

import (
	"context"
	"strings"
	"time"

	"ai-rag-demo/internal/conf"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

func Tracing(cfg *conf.Config) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if cfg.Source.OTel == nil || !cfg.Source.OTel.Enable {
				return handler(ctx, req)
			}
			startTime := time.Now()

			tp, ok := transport.FromServerContext(ctx)
			var requestID string
			if ok {
				requestID = tp.RequestHeader().Get("X-Request-ID")
			}
			if requestID == "" {
				requestID = "req-" + uuid.New().String()
			}

			span := trace.SpanFromContext(ctx)
			if span != nil && span.IsRecording() {
				span.SetAttributes(
					attribute.String("request_id", requestID),
					semconv.RPCSystemKey.String(tp.Kind().String()),
					semconv.RPCServiceKey.String(tp.Operation()),
				)
			}

			reply, err = handler(ctx, req)

			latency := time.Since(startTime)
			if err != nil && span != nil && span.IsRecording() {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			if span != nil && span.IsRecording() {
				span.SetAttributes(attribute.Int64("duration_ms", latency.Milliseconds()))
			}

			return
		}
	}
}
```

---

### 2. ChatModel 调用的 TracedFetcher 封装 (`chat_model/traced_fetcher.go`)

在 `internal/biz/nocli/openai/chat_model` 包中提供 OTel Tracer 装饰器：

```go
package chat_model

import (
	"context"
	"time"

	"ai-rag-demo/internal/biz/nocli/openai/agent/base"

	openai "github.com/sashabaranov/go-openai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ai-rag-demo/llm")

// WrapWithTracing 将 MessageFetcher 装饰上 OpenTelemetry Tracing 能力
func WrapWithTracing(rawFetcher base.MessageFetcher) base.MessageFetcher {
	return func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionMessage, error) {
		ctx, span := tracer.Start(ctx, "gen_ai.chat_model.generate",
			trace.WithSpanKind(trace.SpanKindClient),
		)
		defer span.End()

		span.SetAttributes(
			attribute.String("gen_ai.system", "openai"),
			attribute.String("gen_ai.request.model", req.Model),
			attribute.Int("gen_ai.request.messages_count", len(req.Messages)),
			attribute.Int("gen_ai.request.tools_count", len(req.Tools)),
		)

		start := time.Now()
		msg, err := rawFetcher(ctx, req)
		duration := time.Since(start)

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return msg, err
		}

		span.SetAttributes(
			attribute.Int64("gen_ai.duration_ms", duration.Milliseconds()),
			attribute.String("gen_ai.completion.role", string(msg.Role)),
			attribute.Int("gen_ai.completion.tool_calls_count", len(msg.ToolCalls)),
		)
		span.SetStatus(codes.Ok, "success")

		return msg, nil
	}
}
```

---

### 3. Tool 执行拦截 Span 注入 (`internal/biz/nocli/openai/tool/tool.go`)

在 `toolRegistry.Call` 处自动录入 Tool 名称、参数与执行耗时：

```go
func (r *Registry) Call(ctx context.Context, toolName, argsJSON string) (string, error) {
	ctx, span := tracer.Start(ctx, "gen_ai.tool.call",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("gen_ai.tool.name", toolName),
			attribute.Int("gen_ai.tool.args_bytes", len(argsJSON)),
		),
	)
	defer span.End()

	start := time.Now()
	res, err := r.execute(ctx, toolName, argsJSON)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	span.SetAttributes(
		attribute.Int64("gen_ai.tool.duration_ms", duration.Milliseconds()),
		attribute.Int("gen_ai.tool.result_bytes", len(res)),
	)
	span.SetStatus(codes.Ok, "success")

	return res, nil
}
```

---

### 4. ContextCompressor 历史压缩节点 Span 记录 (`compressor.go`)

在 `Compress` 触发点记录压缩释放效果：

```go
ctx, span := tracer.Start(ctx, "gen_ai.context_compressor",
	trace.WithSpanKind(trace.SpanKindInternal),
	trace.WithAttributes(
		attribute.Int("gen_ai.compress.original_tokens", compRes.OriginalTokens),
		attribute.Int("gen_ai.compress.compressed_tokens", compRes.CompressedTokens),
		attribute.Int("gen_ai.compress.saved_tokens", compRes.OriginalTokens-compRes.CompressedTokens),
	),
)
defer span.End()
```

---

## 七、 监控分析与运维面板集成 (Grafana & Jaeger)

结合本项目在 `config.local.yaml` 中已有的 `otel` 配置：

```yaml
source:
  otel:
    enable: true
    endpoint: "localhost:4317"               # OpenTelemetry Collector gRPC 端口
    sample_rate: 1.0                         # 采样率 (1.0 = 100% 全量采样)
    std_out: false                           # 是否在标准输出打印 Span
    timeout: 5s
```

### 运维收益：
1. **Jaeger 链路追踪可视化**：直接检索 `request_id` 或 `session_id`，查看完整的 ReAct 树状执行图。
2. **Prometheus / Grafana 监控告警**：
   - 告警指标 1：`gen_ai.chat_model.generate` P99 耗时 > 10 秒告警。
   - 告警指标 2：`gen_ai.context_compressor` 压缩触发频率异常攀升告警。
   - 告警指标 3：Agent 达到 `max_iterations` 或触发 `handleTimeoutReached` 的错误占比告警。
