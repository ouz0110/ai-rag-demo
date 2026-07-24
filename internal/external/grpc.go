package external

import (
	"context"
	"fmt"
	"strconv"
	"time"

	cmpb "ai-rag-demo/api/common/v1"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/metrics"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/utils"

	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var canRetryGrpcErrCodes = map[codes.Code]struct{}{
	codes.Unavailable:       {},
	codes.DeadlineExceeded:  {},
	codes.ResourceExhausted: {},
}

type RealTimeDiscoveryGrpcClientConn struct {
	cfg *conf.Config
	cli naming_client.INamingClient
}

func NewRealTimeDiscoveryGrpcClientConn(cfg *conf.Config, cli naming_client.INamingClient) *RealTimeDiscoveryGrpcClientConn {
	return &RealTimeDiscoveryGrpcClientConn{cfg: cfg, cli: cli}
}

func (c *RealTimeDiscoveryGrpcClientConn) getEndpoint(method string) (string, error) {
	v := conf.GetRPCServiceNameByMethod(method)
	var endpoint string
	if conf.IsLocalEnv() {
		endpoint = v
	} else {
		instance, err := c.cli.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{ServiceName: v})
		if err != nil {
			return "", cmpb.ErrorExternalServerError("selectOneHealthyInstance for grpcMethod(%s) error", method).WithCause(err)
		}
		endpoint = fmt.Sprintf("%s:%d", instance.Ip, instance.Port)
	}

	return endpoint, nil
}
func (c *RealTimeDiscoveryGrpcClientConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	invoke := func() (bool, error) {
		endpoint, err := c.getEndpoint(method)
		if err != nil {
			return true, err
		}
		rpcCtx, rpcCancel := context.WithTimeout(ctx, c.cfg.Server.Grpc.OutboundTimeout.Duration)
		defer rpcCancel()
		ncc, err := kratosgrpc.DialInsecure(rpcCtx,
			kratosgrpc.WithOptions(grpc.WithDisableHealthCheck()),
			kratosgrpc.WithEndpoint(endpoint),
			kratosgrpc.WithTimeout(c.cfg.Server.Grpc.OutboundTimeout.Duration),
			kratosgrpc.WithUnaryInterceptor(
				setMetrics,
				logging,
			),
		)
		if err != nil {
			return true, cmpb.ErrorExternalServerError("discovererProblemWorkaround for grpcMethod(%s) error", method).WithCause(err)
		}
		defer ncc.Close()

		err = ncc.Invoke(rpcCtx, method, args, reply, opts...)
		st, _ := status.FromError(err)
		if st != nil {
			if _, ok := canRetryGrpcErrCodes[st.Code()]; ok {
				return true, err
			}
		}
		return false, err
	}
	var err error
	var retry bool
	for i := 0; i < 3; i++ {
		retry, err = invoke()
		if err == nil {
			return nil
		}
		// 重试
		log.Warnw(ctx, "invoke error", "err", err, "grpcMethod", method, "retry_time", i+1)
		if !retry {
			break
		}
		time.Sleep(1 * time.Second)
	}
	log.Errorw(ctx,
		"grpc failed",
		"error", err,
		"grpcMethod", method,
	)
	return err
}

func (c *RealTimeDiscoveryGrpcClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	// TODO implement me
	panic("implement me")
}

func logging(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	reqJson := utils.IgnoreErrorJSONMarshal(req)
	log.Infow(ctx, "rpc start",
		"grpcMethod", method,
		"req", reqJson,
	)

	startTime := time.Now()
	incomingMd, _ := metadata.FromIncomingContext(ctx)
	err := invoker(ctx, method, req, reply, cc, opts...)

	latency := time.Since(startTime)
	if latency > time.Millisecond*200 {
		log.Warnw(ctx,
			fmt.Sprintf("high latency: %v", latency),
			"grpcMethod", method,
		)
	}
	outgoingMd, _ := metadata.FromOutgoingContext(ctx)
	replyJson := utils.IgnoreErrorJSONMarshal(reply)
	if err != nil {
		// 由调用处决定错误处理，这里只打 WARN 日志
		log.Warnw(ctx,
			fmt.Sprintf("rpc failed: %v", err),
			"error", err,
			"grpcMethod", method,
			"req", reqJson,
			"incomingMd", incomingMd,
			"resp", replyJson,
			"outgoingMd", outgoingMd,
			"latency", fmt.Sprint(latency),
		)
		return err
	}

	log.Infow(ctx,
		"rpc finished",
		"grpcMethod", method,
		"req", reqJson,
		"incomingMd", incomingMd,
		"resp", replyJson,
		"outgoingMd", outgoingMd,
		"latency", fmt.Sprint(latency),
	)

	return nil
}

func setMetrics(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	startTime := time.Now()
	err := invoker(ctx, method, req, reply, cc, opts...)
	var code codes.Code
	if err != nil {
		gs, _ := status.FromError(err)
		code = gs.Code()
	}
	latency := time.Since(startTime)
	// RpcCount, RpcLatency 参考 github.com/go-kratos/kratos/v2@v2.7.0/middleware/metrics/metrics.go:39
	metrics.RpcCount.WithLabelValues(method, strconv.Itoa(int(code)), code.String()).Inc()
	metrics.RpcLatency.WithLabelValues(method).Observe(latency.Seconds())

	return err
}
