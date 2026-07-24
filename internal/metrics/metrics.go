package metrics

import (
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/prometheus/client_golang/prometheus"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

var (
	RequestCount   *prometheus.CounterVec   // 请求数
	RequestLatency *prometheus.HistogramVec // 请求延迟
	RpcCount       *prometheus.CounterVec   // rpc 请求数
	RpcLatency     *prometheus.HistogramVec // rpc 请求延迟
	LogCount       *prometheus.CounterVec   // 日志数
)

func init() {
	RequestLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "server",
		Subsystem: "requests",
		Name:      "duration_sec",
		Help:      "server requests duration(sec).",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.250, 0.5, 1},
	}, []string{"kind", "operation"})
	RequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "client",
		Subsystem: "requests",
		Name:      "code_total",
		Help:      "The total number of processed requests",
	}, []string{"kind", "operation", "code", "reason"})
	RpcCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "server",
		Subsystem: "rpc_calls",
		Name:      "code_total",
		Help:      "The total number of rpc calls",
	}, []string{"operation", "code", "reason"})
	RpcLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "server",
		Subsystem: "rpc_calls",
		Name:      "duration_sec",
		Help:      "rpc calls duration(sec).",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.250, 0.5, 1},
	}, []string{"operation"})
	LogCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "server",
		Subsystem: "log",
		Name:      "level_total",
		Help:      "The total number of logs",
	}, []string{"name", "level"})
}

func Register() {
	prometheus.MustRegister(
		RequestLatency, RequestCount,
		RpcCount, RpcLatency, LogCount,
	)
}

// MetricSeconds is the OpenTelemetry histogram instrument for server latency (kratos >= 2.8.0).
var MetricSeconds metric.Float64Histogram

// MetricRequests is the OpenTelemetry counter instrument for server requests (kratos >= 2.8.0).
var MetricRequests metric.Int64Counter

// InitMetrics initializes OpenTelemetry metric instruments for the kratos metrics middleware.
func InitMetrics(name string) {
	exporter, err := otelprom.New()
	if err != nil {
		panic(err)
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	meter := provider.Meter(name)

	MetricRequests, err = metrics.DefaultRequestsCounter(meter, metrics.DefaultServerRequestsCounterName)
	if err != nil {
		panic(err)
	}
	MetricSeconds, err = metrics.DefaultSecondsHistogram(meter, metrics.DefaultServerSecondsHistogramName)
	if err != nil {
		panic(err)
	}
}
