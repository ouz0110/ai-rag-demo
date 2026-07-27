package server

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	basepb "ai-rag-demo/api/base/v1"
	cmpb "ai-rag-demo/api/common/v1"
	noclipb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/cache"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/metrics"
	"ai-rag-demo/internal/pkg/i18n"
	"ai-rag-demo/internal/pkg/utils"
	"ai-rag-demo/internal/server/middleware"
	"ai-rag-demo/internal/service/base"
	"ai-rag-demo/internal/service/nocli"

	_ "net/http/pprof" // pprof support

	"github.com/go-kratos/kratos/v2/errors"
	kratosmetrics "github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	transHttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/gorilla/handlers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	allowedMethod  = []string{"GET", "POST", "PATCH", "PUT", "HEAD", "OPTIONS", "DELETE"}
	allowedHeaders = []string{
		"Content-Type", "Authorization", "Accept-Language", "Access-Control-Allow-Origin",
		"X-Requested-With", "X-Request-ID", "X-Forwarded-For", "remote_addr",
		"Platform", "Phone-Type", "Sm-Device-Id", "App-Version",
		"Nonce", "Timestamp", "Signature",
	}
)

// NewHTTPServer new a HTTP server.
func NewHTTPServer(
	cfg *conf.Config,
	cache *cache.Cache,
	accountSrv *base.AccountService,
	chatSrv *nocli.ChatService,
	kbSrv *nocli.KBService,
) *transHttp.Server {
	opts := wrapHTTPOptions(cfg, cache)
	server := transHttp.NewServer(opts...)

	server.Handle("/metrics", promhttp.Handler())
	rootRouter := server.Route("/")
	rootRouter.GET("/live", func(ctx transHttp.Context) error { return ctx.Result(http.StatusOK, nil) })
	rootRouter.GET("/ready", func(ctx transHttp.Context) error { return ctx.Result(http.StatusOK, nil) })

	// 🎯 显式注册 HTTP SSE 流式推导与恢复接口 (经过中间件链处理，确保包含鉴权 UserFromContext、日志与 Trace)
	rootRouter.POST("/nocli/v1/stream/completion", wrapStreamHandler(cfg, cache, chatSrv.StreamCompletionHTTP))
	rootRouter.POST("/nocli/v1/stream/resume", wrapStreamHandler(cfg, cache, chatSrv.StreamResumeHTTP))

	// 🎯 注册独立文件上传 HTTP 接口 (经过 Kratos 全量中间件链处理，确保包含鉴权 UserFromContext、日志与 Trace)
	rootRouter.POST("/nocli/v1/rag/upload", wrapHTTPHandler(kbSrv.UploadFileHTTP))

	// 注册账号服务
	basepb.RegisterAccountsHTTPServer(server, accountSrv)

	// 注册 AI 对话服务与 Protobuf 知识库管理服务 (常规 Unary HTTP 接口)
	noclipb.RegisterNocliChatHTTPServer(server, chatSrv)
	noclipb.RegisterKnowledgeBaseHTTPServer(server, kbSrv)

	return server
}

func wrapHTTPOptions(cfg *conf.Config, cache *cache.Cache) []transHttp.ServerOption {
	opts := []transHttp.ServerOption{
		transHttp.Middleware(
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
		),
		transHttp.Filter(
			handlers.CORS(
				handlers.AllowedHeaders(allowedHeaders),
				handlers.AllowedMethods(allowedMethod),
				handlers.AllowedOrigins([]string{"*"}),
				handlers.AllowCredentials(),
			),
		),
		transHttp.RequestQueryDecoder(decodeRequestQuery),
		transHttp.ResponseEncoder(encodeResponse),
		transHttp.ErrorEncoder(encodeErrorResponse),
	}
	httpCfg := cfg.Server.HTTP
	if httpCfg.Addr != "" {
		opts = append(opts, transHttp.Address(httpCfg.Addr))
	}
	if httpCfg.Timeout.Duration != 0 {
		opts = append(opts, transHttp.Timeout(httpCfg.Timeout.Duration))
	}

	return opts
}

func decodeRequestQuery(r *http.Request, in interface{}) error {
	newQuery := url.Values{}
	for k, vv := range r.URL.Query() {
		for _, v := range vv {
			// 移除首尾空格
			// 数字带空格时 unmarshal 进 proto 整型会 panic。
			newQuery.Add(k, strings.TrimSpace(v))
		}
	}

	r.URL.RawQuery = newQuery.Encode()

	return transHttp.DefaultRequestQuery(r, in)
}

func encodeResponse(w http.ResponseWriter, r *http.Request, v interface{}) error {
	if _, ok := v.(transHttp.Redirector); ok {
		return transHttp.DefaultResponseEncoder(w, r, v)
	}

	res := &cmpb.Response{
		Code:      0,
		Message:   "ok",
		RequestId: getRequestId(w, r),
	}
	var err error
	if v != nil && !reflect.ValueOf(v).IsNil() {
		res.Data, err = utils.ConvertToProtoStruct(v)
		if err != nil {
			return err
		}
	}
	return transHttp.DefaultResponseEncoder(w, r, res)
}

func getRequestId(w http.ResponseWriter, r *http.Request) string {
	if s := r.Header.Get("X-Request-Id"); s != "" {
		return s
	}
	if s := w.Header().Get("X-Request-Id"); s != "" {
		return s
	}
	return ""
}

// encodeErrorResponse 参考 transHttp.DefaultErrorEncoder 进行改造。
func encodeErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	se := errors.FromError(err)
	httpStatus := int(se.Code)

	res := &cmpb.Response{}
	res.RequestId = getRequestId(w, r)
	errorEnum := utils.ProtoEnumByName[cmpb.ErrorEnum](se.Reason)
	if errorEnum == 0 {
		res.Code = int32(cmpb.ErrorEnum_UNKNOWN_ERROR)
	} else {
		res.Code = int32(errorEnum)
	}
	// message
	res.Message = getErrorMsg(r, se)
	// cause
	if conf.IsTestEnv() {
		cause := err.Error()
		res.Cause = &cause
	}
	res.Metadata = se.Metadata
	res.Reason = se.Reason

	codec, _ := transHttp.CodecForRequest(r, "Accept")
	body, err := codec.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", fmt.Sprintf("application/%s", codec.Name()))
	w.WriteHeader(httpStatus)
	_, _ = w.Write(body)
}

func getErrorMsg(r *http.Request, err *errors.Error) string {
	accept := r.Header.Get("accept-language")
	localizer := i18n.NewLocalizer(accept)
	ctx := i18n.WithLocalizer(r.Context(), localizer)
	if err.Message != "" {
		msg := i18n.NoErr.GetMessage(ctx, err.Message)
		if msg != "" {
			return msg
		}
		return err.Message
	}
	// reason 作为 id
	i18nMsgID := "error." + strings.ToLower(err.Reason)
	msg := i18n.NoErr.GetMessage(ctx, i18nMsgID)
	if msg != "" {
		return msg
	}
	// 默认值
	switch err.Code {
	case 500:
		i18nMsgID = "error.internal_server_error"
	default:
		i18nMsgID = "error.unknown_error"
	}
	return i18n.NoErr.GetMessage(ctx, i18nMsgID)
}
