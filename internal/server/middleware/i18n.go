package middleware

import (
	"ai-rag-demo/internal/common"
	"context"

	"ai-rag-demo/internal/pkg/i18n"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

func I18N() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			header, _ := transport.FromServerContext(ctx)
			accept := header.RequestHeader().Get("accept-language")
			localizer := i18n.NewLocalizer(accept)
			ctx = i18n.WithLocalizer(ctx, localizer)
			ctx = common.WithLanguage(ctx, accept)
			return handler(ctx, req)
		}
	}
}
