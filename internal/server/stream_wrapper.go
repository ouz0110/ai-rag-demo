package server

import (
	"context"
	"net/http"

	"ai-rag-demo/internal/cache"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/metrics"
	"ai-rag-demo/internal/server/middleware"

	kratosmw "github.com/go-kratos/kratos/v2/middleware"
	kratosmetrics "github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport"
	transHttp "github.com/go-kratos/kratos/v2/transport/http"
)

// wrapStreamHandler 为自定义的 SSE 流式 Endpoint 包裹全量 Kratos 中间件链 (鉴权, 日志, Trace, i18n 等)
func wrapStreamHandler(
	cfg *conf.Config,
	cache *cache.Cache,
	handler func(w http.ResponseWriter, r *http.Request),
) func(ctx transHttp.Context) error {
	mChain := kratosmw.Chain(
		kratosmetrics.Server(
			kratosmetrics.WithSeconds(metrics.MetricSeconds),
			kratosmetrics.WithRequests(metrics.MetricRequests),
		),
		middleware.Logging(),
		middleware.ErrorWrapper(),
		middleware.Recovery(),
		tracing.Server(),
		middleware.Tracing(cfg),
		validate.Validator(),
		middleware.I18N(),
		middleware.HTTPAuth(cfg, cache),
	)

	return func(ctx transHttp.Context) error {
		req := ctx.Request()
		resp := ctx.Response()

		// 构造符合 Kratos transport.Transporter 接口的上下文对象，供 HTTPAuth 提取 Authorization Header 与 Operation
		tr := &streamTransport{req: req}
		trCtx := transport.NewServerContext(req.Context(), tr)

		h := mChain(func(mwCtx context.Context, req interface{}) (interface{}, error) {
			httpReq := req.(*http.Request).WithContext(mwCtx)
			handler(resp, httpReq)
			return nil, nil
		})

		if _, err := h(trCtx, req); err != nil {
			encodeErrorResponse(resp, req, err)
			return nil
		}
		return nil
	}
}

// wrapHTTPHandler 为自定义的 HTTP Context Handler 挂载全量 Kratos 中间件链 (自动提取 Operation，包含鉴权, 日志, Trace, i18n, Recovery 等)
func wrapHTTPHandler(handler func(ctx transHttp.Context) error) func(ctx transHttp.Context) error {
	return func(ctx transHttp.Context) error {
		req := ctx.Request()
		// 动态获取当前请求的 Path 作为 Operation，无需手动硬编码重复传参
		transHttp.SetOperation(ctx, req.URL.Path)
		h := ctx.Middleware(func(mwCtx context.Context, _ interface{}) (interface{}, error) {
			// 将经过中间件链 (包含 HTTPAuth 注入的用户 context) 重新绑定给 request
			*req = *req.WithContext(mwCtx)
			return nil, handler(ctx)
		})
		_, err := h(req.Context(), nil)
		return err
	}
}

// streamTransport 为 SSE 流式请求实现 transport.Transporter 接口
type streamTransport struct {
	req *http.Request
}

func (s *streamTransport) Kind() transport.Kind { return transport.KindHTTP }
func (s *streamTransport) Endpoint() string     { return s.req.URL.Path }
func (s *streamTransport) Operation() string    { return s.req.URL.Path }
func (s *streamTransport) RequestHeader() transport.Header {
	return streamHeader(s.req.Header)
}
func (s *streamTransport) ReplyHeader() transport.Header {
	return streamHeader(http.Header{})
}

// streamHeader 适配 transport.Header 接口
type streamHeader http.Header

func (h streamHeader) Get(key string) string  { return http.Header(h).Get(key) }
func (h streamHeader) Set(key, value string) { http.Header(h).Set(key, value) }
func (h streamHeader) Add(key, value string) { http.Header(h).Add(key, value) }
func (h streamHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}
func (h streamHeader) Values(key string) []string { return http.Header(h).Values(key) }
