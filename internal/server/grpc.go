package server

import (
	noclipb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/metrics"
	"ai-rag-demo/internal/server/middleware"
	"ai-rag-demo/internal/service/nocli"

	kratosmetrics "github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(
	cfg *conf.Config,
	kbSrv *nocli.KBService,
) *grpc.Server {
	opts := wrapGRPCOptions(cfg)
	server := grpc.NewServer(opts...)

	// 注册 Protobuf 知识库管理 gRPC 服务
	noclipb.RegisterKnowledgeBaseServer(server, kbSrv)

	return server
}

func wrapGRPCOptions(cfg *conf.Config) []grpc.ServerOption {
	opts := []grpc.ServerOption{
		grpc.Middleware(
			kratosmetrics.Server(
				kratosmetrics.WithSeconds(metrics.MetricSeconds),
				kratosmetrics.WithRequests(metrics.MetricRequests),
			),
			middleware.ErrorWrapper(),
			middleware.Recovery(),
			tracing.Server(),
			middleware.Logging(),
			middleware.Tracing(cfg),
			validate.Validator(),
		),
	}
	grpcCfg := cfg.Server.Grpc
	if grpcCfg.Addr != "" {
		opts = append(opts, grpc.Address(grpcCfg.Addr))
	}
	if grpcCfg.Timeout.Duration != 0 {
		opts = append(opts, grpc.Timeout(grpcCfg.Timeout.Duration))
	}

	return opts
}
