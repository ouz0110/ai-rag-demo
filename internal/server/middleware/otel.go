package middleware

import (
	"context"
	"strings"
	"time"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/observability"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// Tracing 为每个请求在整个请求链路中共享同一个 span，并补充详细属性、错误标记和耗时。
// 依赖 kratos tracing.Server() 已在外部创建顶层 span，此处负责 span 属性补充。
func Tracing(cfg *conf.Config) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			startTime := time.Now()
			var requestID string
			if tp, ok := transport.FromServerContext(ctx); ok && tp.RequestHeader() != nil {
				requestID = tp.RequestHeader().Get("X-Request-ID")
			}
			ctx = observability.WithRequestID(ctx, requestID)
			requestID = observability.GetRequestID(ctx)

			// 🎯 观测者开关控制：
			// 1. 本地日志链路开关 (enable_trace_log)：开启后在本地日志打印全链路切面节点 (LogObserver)；
			// 2. OpenTelemetry 远程导出开关 (otel.enable)：开启后向 OTel Collector 推送 Trace (OTelObserver)。
			var observers []observability.Observer
			if cfg != nil && cfg.Source.Log.EnableTraceLog {
				observers = append(observers, observability.NewLogObserver())
			}
			if cfg != nil && cfg.Source.OTel != nil && cfg.Source.OTel.Enable {
				observers = append(observers, observability.NewOTelObserver(nil))
			}
			if len(observers) > 0 {
				ctx = observability.WithObserver(ctx, observability.NewCompositeObserver(observers...))
			}

			span := trace.SpanFromContext(ctx)
			createdSelf := false
			if (span == nil || !span.IsRecording()) && (cfg != nil && cfg.Source.OTel != nil && cfg.Source.OTel.Enable) {
				operation := "server.request"
				if tp, ok := transport.FromServerContext(ctx); ok {
					operation = tp.Operation()
				}
				ctx, span = otel.Tracer("ai-rag-demo/server").Start(ctx, operation, trace.WithSpanKind(trace.SpanKindServer))
				createdSelf = true
			}

			if createdSelf {
				defer span.End()
			}

			if span != nil && span.IsRecording() {
				tp, _ := transport.FromServerContext(ctx)
				operation := tp.Operation()
				kind := tp.Kind().String()
				var peer string
				switch kind {
				case "HTTP":
					peer = peerAddrHTTP(ctx)
				case "gRPC":
					peer = peerAddrGRPC(ctx)
				}

				span.SetAttributes(
					attribute.String("request_id", requestID),
					semconv.RPCSystemKey.String(kind),
					semconv.RPCServiceKey.String(operation),
					semconv.PeerServiceKey.String(peer),
					semconv.NetworkPeerAddressKey.String(peer),
				)
			}

			reply, err = handler(ctx, req)

			latency := time.Since(startTime)
			if err != nil && span != nil && span.IsRecording() {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			if span != nil && span.IsRecording() {
				span.SetAttributes(semconv.FaaSTimeKey.Float64(float64(latency.Milliseconds())))
			}
			return
		}
	}
}

func peerAddrHTTP(ctx context.Context) string {
	if tp, ok := transport.FromServerContext(ctx); ok {
		if h := tp.RequestHeader().Get("X-Forwarded-For"); h != "" {
			return strings.Split(h, ",")[0]
		}
		if h := tp.RequestHeader().Get("X-Real-IP"); h != "" {
			return h
		}
		if p := tp.RequestHeader().Get("remote_addr"); p != "" {
			return p
		}
	}
	return clientIPFromCtx(ctx)
}

func peerAddrGRPC(ctx context.Context) string {
	if client, ok := transport.FromServerContext(ctx); ok {
		if peer := client.RequestHeader().Get("X-Forwarded-For"); peer != "" {
			return strings.Split(peer, ",")[0]
		}
	}
	return clientIPFromCtx(ctx)
}

func clientIPFromCtx(ctx context.Context) string {
	if ip, ok := ctx.Value("client_ip").(string); ok && ip != "" {
		return ip
	}
	return ""
}
