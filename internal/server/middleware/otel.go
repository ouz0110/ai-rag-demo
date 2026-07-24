package middleware

import (
	"context"
	"strings"
	"time"

	"ai-rag-demo/internal/conf"

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
			if cfg.Source.OTel == nil || !cfg.Source.OTel.Enable {
				return handler(ctx, req)
			}
			startTime := time.Now()

			tp, _ := transport.FromServerContext(ctx)
			operation := tp.Operation()
			kind := tp.Kind().String()

			span := trace.SpanFromContext(ctx)
			if span == nil || !span.IsRecording() {
				return handler(ctx, req)
			}

			var peer string
			switch kind {
			case "HTTP":
				peer = peerAddrHTTP(ctx)
			case "gRPC":
				peer = peerAddrGRPC(ctx)
			}

			span.SetAttributes(
				semconv.RPCSystemKey.String(kind),
				semconv.RPCServiceKey.String(operation),
				semconv.PeerServiceKey.String(peer),
				semconv.NetworkPeerAddressKey.String(peer),
			)

			reply, err = handler(ctx, req)

			latency := time.Since(startTime)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			span.SetAttributes(semconv.FaaSTimeKey.Float64(float64(latency.Milliseconds())))

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
