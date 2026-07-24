package middleware

import (
	"context"
	"net"
	"strings"
	"time"

	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/utils"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var highAlarmSeverityList = map[string]struct{}{}

// Logging 为请求生成 logger 并注入到 ctx，同时打印请求详情。
// 参考: logging.Server
func Logging() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			tp, _ := transport.FromServerContext(ctx)
			operation := tp.Operation()
			kind := tp.Kind().String()
			requestHeader := tp.RequestHeader()
			requestID := requestHeader.Get("X-Request-Id")
			if requestID == "" {
				requestID = utils.NewUUID()
			}
			replyHeader := tp.ReplyHeader()
			replyHeader.Add("X-Request-Id", requestID)

			requestLogger := log.NewLogger("request").With(
				zap.String("trace", trace.SpanContextFromContext(ctx).TraceID().String()),
				zap.String("span", trace.SpanContextFromContext(ctx).SpanID().String()),
				zap.String("requestID", requestID),
				zap.String("operation", operation),
				zap.String("kind", kind),
			)

			// 将 logger 注入 ctx，以使用 pkg/log/ 内方法打日志。
			ctx = log.WithLogger(ctx, requestLogger)
			xForwardedFor := requestHeader.Get("X-Original-Forwarded-For")
			if len(xForwardedFor) == 0 {
				xForwardedFor = requestHeader.Get("X-Forwarded-For")
			}

			// 打日志。
			reqJson := utils.IgnoreErrorJSONMarshal(req)
			log.Infow(ctx, "request received",
				"协议", kind,
				"网络环境", requestHeader.Get("Net-Type"),
				"IMEI", requestHeader.Get("IMEI"),
				"APP版本", requestHeader.Get("App-Version"),
				"JWT", requestHeader.Get("Authorization"),
				"目标IP", outBoundIP(ctx),
				"目标PORT", 8000,
				"目标URI", operation,
				"请求IP", xForwardedFor,
				"请求端口", 80,
				"requestHeader", requestHeader,
				"req", reqJson,
			)

			startTime := time.Now()
			reply, err = handler(ctx, req)
			latency := time.Since(startTime)

			if latency > time.Millisecond*500 {
				log.Warnf(ctx, "high latency: %s", latency.String())
			}
			if err != nil {
				se := errors.FromError(err)
				if se != nil && se.Code == errors.UnknownCode {
					log.Warnw(ctx,
						"request failed",
						"err", err,
						"requestHeader", requestHeader,
						"req", reqJson,
						"latency", latency.String(),
					)
				}
				return
			}
			jsonReply := utils.IgnoreErrorJSONMarshal(reply)
			log.Infow(ctx,
				"request finished",
				"requestHeader", requestHeader,
				"req", reqJson,
				"reply", jsonReply,
				"latency", latency.String(),
			)
			return
		}
	}
}

func outBoundIP(ctx context.Context) string {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		log.Warnf(ctx, "get outBoundIP error: %v", err)
		return ""
	}

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return strings.Split(localAddr.String(), ":")[0]
}
