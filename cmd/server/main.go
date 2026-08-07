package main

import (
	"context"
	"flag"
	"os"
	"time"

	"ai-rag-demo/internal/cache"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/metrics"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/utils"

	"github.com/go-kratos/kratos/contrib/registry/nacos/v2"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	Name      string
	Version   string
	Revision  string // commit hash。
	BuildTime string // 构建时间。

	startTime = time.Now() // 服务启动时间。
	id, _     = os.Hostname()

	// flagconf is the config flag.
	flagconf string
)

func init() {
	flag.StringVar(&flagconf, "conf", "configs/config.local.yaml", "config path, eg: -conf config.yaml")
}

type app struct {
	kratosApp *kratos.App
}

func newApp(
	gs *grpc.Server,
	hs *http.Server,
	registry *nacos.Registry,
	cfg *conf.Config,
	rdb *cache.Cache,
) *app {
	var ops []kratos.Option
	ops = append(ops,
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{
			"revision":   Revision,
			"build_time": BuildTime,
			"start_time": startTime.Format(time.RFC3339),
		}),
		kratos.Logger(log.NewKratosLogger("kratos")),
		kratos.Server(gs, hs),
	)
	if !conf.IsLocalEnv() {
		ops = append(ops, kratos.Registrar(registry))
	}
	utils.JWTInit(cfg.Secret)
	return &app{
		kratosApp: kratos.New(ops...),
	}
}

func main() {
	flag.Parse()

	config := prepare()
	metrics.InitMetrics(Name)
	a, cleanup, err := wireApp(config)
	if err != nil {
		panic(err)
	}
	defer cleanup()
	metrics.Register()

	ctx := context.Background()
	if config.Source.OTel != nil && config.Source.OTel.Enable {
		otelTimeout := config.Source.OTel.Timeout.Duration
		shutdown, err := utils.InitTracer(ctx, utils.TracerConfig{
			ServerName: config.Name,
			Endpoint:   config.Source.OTel.Endpoint,
			SampleRate: config.Source.OTel.SampleRate,
			StdOut:     config.Source.OTel.StdOut,
			Timeout:    otelTimeout,
		})
		if err != nil {
			os.Exit(1)
		}
		if shutdown != nil {
			defer func() {
				_ = shutdown(ctx)
			}()
		}
	}

	// start and wait for stop signal
	if err = a.kratosApp.Run(); err != nil {
		panic(err)
	}
}
