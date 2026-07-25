package log

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	gormlogger "gorm.io/gorm/logger"
	"moul.io/zapgorm2"
)

var (
	// 根 logger，所有 logger 都应该基于 rootLogger
	rootLogger *zap.Logger

	DefaultLogger *zap.Logger
)

func Init(level string, opts ...zap.Option) {
	encoder := getEncoder()
	l, _ := zapcore.ParseLevel(level)
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), l),
		zapcore.NewCore(encoder, newFileWriteSyncer(&fileLogConfig{
			Name:       "log/error.log",
			MaxAge:     30, // 30 天
			MaxSize:    50, // 50 M
			MaxBackups: 6,  // 6 个备份文件
		}), zapcore.ErrorLevel),
	)
	opts = append(opts,
		zap.AddCallerSkip(1), // 打日志均通过 log.go 下方法，因此 skip 加 1（直接调用 DefaultLogger 除外）
	)
	rootLogger = zap.New(core, zap.AddCaller()).WithOptions(opts...)
	DefaultLogger = rootLogger.Named("default")
}

type fileLogConfig struct {
	Name       string
	MaxAge     int
	MaxSize    int
	MaxBackups int
}

func newFileWriteSyncer(cfg *fileLogConfig) zapcore.WriteSyncer {
	return zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.Name,
		MaxAge:     cfg.MaxAge,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
	})
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.MessageKey = messageKey
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

func NewLogger(name string) *zap.Logger {
	return rootLogger.Named(name)
}

type KratosLogger struct {
	l *zap.Logger
}

func NewKratosLogger(name string) *KratosLogger {
	return &KratosLogger{l: NewLogger(name).WithOptions(zap.AddCallerSkip(1))}
}

func (l *KratosLogger) Log(level log.Level, kv ...interface{}) error {
	fields, msgFields := mergeKeysAndValues(kv)

	var msg string
	if c := len(msgFields); c == 1 {
		msg = msgFields[0].String
	} else if c > 1 {
		var ss []string
		for _, v := range msgFields {
			ss = append(ss, v.String)
		}
		msg = fmt.Sprintf("multiple message combined: %s", strings.Join(ss, ", "))
		l.l.Warn("multiple message field")
	}

	l.l.Log(zapcore.Level(level), msg, fields...)

	return nil
}

type NacosLogger struct {
	*zap.SugaredLogger
}

func NewNacosLogger(name, level string) *NacosLogger {
	// 此时 DefaultLogger 还未初始化
	writeSyncer := newFileWriteSyncer(&fileLogConfig{
		Name:       "log/nacos.log",
		MaxAge:     30, // 30 天
		MaxSize:    50, // 50 M
		MaxBackups: 6,  // 6 个备份文件
	})
	encoder := getEncoder()
	l, _ := zapcore.ParseLevel(level)
	core := zapcore.NewCore(encoder, writeSyncer, l)

	return &NacosLogger{zap.New(core, zap.AddCaller()).Named(name).Sugar()}
}

type GormLogger struct {
	zapgorm2.Logger
}

func NewGormLogger(name string) *GormLogger {
	l := zapgorm2.New(NewLogger(name).WithOptions(zap.AddCallerSkip(-1)))
	switch rootLogger.Level() {
	case zap.DebugLevel, zap.InfoLevel:
		l.LogLevel = gormlogger.Info // zapgorm2 实际会打印成 debug 级别，见：moul.io/zapgorm2@v1.3.0/zapgorm2.go:54
	case zap.WarnLevel:
		l.LogLevel = gormlogger.Warn
	default:
		l.LogLevel = gormlogger.Error
	}
	l.IgnoreRecordNotFoundError = true

	return &GormLogger{l}
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	// 获取 SQL 语句和受影响行数
	sql, rows := fc()
	elapsed := time.Since(begin)

	logger := LoggerFromContext(ctx)
	if logger == nil {
		logger = l.ZapLogger
	}
	// 注意：如果你还想保留 GORM 原有的错误处理逻辑，可以参考源码做更多判断，这里仅做演示
	fields, msgFields := mergeKeysAndValues([]interface{}{
		"sql", sql,
		"row", rows,
		"elapesd", float64(elapsed.Nanoseconds()) / 1e6,
	})

	var msg string
	if c := len(msgFields); c == 1 {
		msg = msgFields[0].String
	} else if c > 1 {
		var ss []string
		for _, v := range msgFields {
			ss = append(ss, v.String)
		}
		msg = fmt.Sprintf("multiple message combined: %s", strings.Join(ss, ", "))
		logger.Warn("multiple message field")
	}

	logger.Log(zapcore.DebugLevel, msg, fields...)
}
