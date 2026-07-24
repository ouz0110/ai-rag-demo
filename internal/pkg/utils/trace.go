package utils

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// TracerConfig OpenTelemetry Trace 配置
type TracerConfig struct {
	ServerName string
	Endpoint   string
	SampleRate float32
	StdOut     bool
}

// InitTracer OpenTelemetry Trace 初始化入口
func InitTracer(ctx context.Context, conf TracerConfig) (func(context.Context) error, error) {
	return doInit(ctx, conf)
}

func doInit(ctx context.Context, conf TracerConfig) (func(context.Context) error, error) {
	var (
		exporter sdktrace.SpanExporter
		err      error
	)
	// 1. 选择 exporter：优先 OTLP gRPC，其次 stdout 调试输出
	switch {
	case conf.Endpoint != "":
		exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(conf.Endpoint),
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithTimeout(5*time.Second),
		)
		if err != nil {
			return nil, err
		}
	case conf.StdOut:
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
	default:
		return func(ctx context.Context) error { return nil }, nil
	}
	// 2. 采样策略：<=0 跳过全部、>1 全部采样、否则按比例采样
	var sampler sdktrace.Sampler
	switch {
	case conf.SampleRate <= 0:
		sampler = sdktrace.NeverSample()
	case conf.SampleRate > 1:
		sampler = sdktrace.AlwaysSample()
	default:
		sampler = sdktrace.TraceIDRatioBased(float64(conf.SampleRate))
	}
	// 3. 构造 TracerProvider：绑定采样器、导出器、服务名资源
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(conf.ServerName),
		)),
	)
	// 4. 注册全局 provider 与传播器（TraceContext + Baggage）
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	// 5. 返回 shutdown 函数，由调用方在退出时清理
	return tp.Shutdown, nil
}

// Tracer 返回指定名称的 Tracer
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
