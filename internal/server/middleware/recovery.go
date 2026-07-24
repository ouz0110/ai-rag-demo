package middleware

import (
	"context"
	"fmt"
	"runtime"
	"time"

	cmpb "ai-rag-demo/api/common/v1"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/utils"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// Recovery is a server middleware that recovers from any panics.
// 参考 github.com/go-kratos/kratos/v2@v2.7.0/middleware/recovery/recovery.go
func Recovery() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			startTime := time.Now()
			defer func() {
				if rerr := recover(); rerr != nil {
					buf := make([]byte, 64<<10) //nolint:gomnd
					n := runtime.Stack(buf, false)
					buf = buf[:n]

					latency := time.Since(startTime)
					var operation, requestID string
					var requestHeader transport.Header
					tp, ok := transport.FromServerContext(ctx)
					if ok {
						operation = tp.Operation()
						requestHeader = tp.RequestHeader()
						requestID = requestHeader.Get("X-Request-Id")
						replyHeader := tp.ReplyHeader()
						replyHeader.Add("X-Request-Id", requestID)
					}
					reqJson := utils.IgnoreErrorJSONMarshal(req)
					log.Errorw(ctx,
						"request panic",
						"err", fmt.Errorf("%v", rerr),
						"operation", operation,
						"requestID", requestID,
						"latency", fmt.Sprint(latency),
						"req", reqJson,
						"requestHeader", requestHeader,
						"stack", string(buf),
					)

					err = cmpb.ErrorPanicError("%v, stack: %s", rerr, string(buf))
				}
			}()

			reply, err = handler(ctx, req)

			return
		}
	}
}
