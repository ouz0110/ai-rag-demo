package common

import (
	"context"
	"runtime"

	"ai-rag-demo/internal/pkg/log"

	"github.com/go-kratos/kratos/v2/transport"
)

func RunInGoroutine(ctx context.Context, run func(ctx context.Context)) {
	logger := log.LoggerFromContext(ctx)
	newCtx := log.WithLogger(context.Background(), logger)
	ok, wrap := UserFromContext(ctx)
	if ok {
		newCtx = WithUser(newCtx, wrap)
	}
	go func(ctx context.Context) {
		defer func() {
			if rerr := recover(); rerr != nil {
				buf := make([]byte, 64<<10) //nolint:gomnd
				n := runtime.Stack(buf, false)
				buf = buf[:n]
				log.Errorf(ctx, "RunInGoroutine panic, stack: %s", string(buf))
			}
		}()

		run(ctx)
	}(newCtx)
}

func ctxHeader(ctx context.Context, header string) string {
	cc, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	return cc.RequestHeader().Get(header)
}
