package middleware

import (
	"context"

	cmpb "ai-rag-demo/api/common/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
)

func ErrorWrapper() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			reply, err = handler(ctx, req)
			if se := errors.FromError(err); se != nil && se.Code == errors.UnknownCode && se.Reason == errors.UnknownReason {
				err = cmpb.ErrorInternalServerError(err.Error(), "")
			}
			return
		}
	}
}
