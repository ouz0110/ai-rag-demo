package log

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

const messageKey = "msg"

type contextKey struct{}

func mergeKeysAndValues(kv []interface{}) (fields []zap.Field, msgFields []zap.Field) {
	if len(kv) == 0 || len(kv)%2 != 0 {
		DefaultLogger.Warn("key value must appear in pairs: ", zap.Any("kv", kv))
		return nil, nil
	}

	for i := 0; i < len(kv); i += 2 {
		key := kv[i].(string)
		value := kv[i+1]
		var f zap.Field
		switch v := value.(type) {
		case string:
			f = zap.String(key, truncateLargeString(v))
		case int:
			f = zap.Int(key, v)
		case bool:
			f = zap.Bool(key, v)
		default:
			f = zap.Any(key, v)
		}
		if key == messageKey {
			msgFields = append(msgFields, f)
		} else {
			fields = append(fields, f)
		}
	}

	return fields, msgFields
}

func truncateLargeString(s string) string {
	const limit = 10 * 1024 // 10kb
	if len([]byte(s)) > limit {
		return s[:limit] + "..."
	}
	return s
}

func WithLogger(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

func LoggerFromContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return DefaultLogger
	}
	if l, ok := ctx.Value(contextKey{}).(*zap.Logger); ok {
		return l
	}

	return DefaultLogger
}

func Debugf(ctx context.Context, format string, a ...interface{}) {
	logger := LoggerFromContext(ctx)
	logger.Log(zap.DebugLevel, fmt.Sprintf(format, a...))
}

func Debugw(ctx context.Context, msg string, keyvals ...interface{}) {
	fields, _ := mergeKeysAndValues(keyvals)
	logger := LoggerFromContext(ctx)
	logger.Log(zap.DebugLevel, msg, fields...)
}

func Infof(ctx context.Context, format string, a ...interface{}) {
	logger := LoggerFromContext(ctx)
	logger.Log(zap.InfoLevel, fmt.Sprintf(format, a...))
}

func Infow(ctx context.Context, msg string, keyvals ...interface{}) {
	fields, _ := mergeKeysAndValues(keyvals)
	logger := LoggerFromContext(ctx)
	logger.Log(zap.InfoLevel, msg, fields...)
}

func Warnf(ctx context.Context, format string, a ...interface{}) {
	logger := LoggerFromContext(ctx)
	logger.Log(zap.WarnLevel, fmt.Sprintf(format, a...))
}

func Warnw(ctx context.Context, msg string, keyvals ...interface{}) {
	fields, _ := mergeKeysAndValues(keyvals)
	logger := LoggerFromContext(ctx)
	logger.Log(zap.WarnLevel, msg, fields...)
}

func Errorf(ctx context.Context, format string, a ...interface{}) {
	logger := LoggerFromContext(ctx)
	logger.Log(zap.ErrorLevel, fmt.Sprintf(format, a...))
}

func Errorw(ctx context.Context, msg string, keyvals ...interface{}) {
	fields, _ := mergeKeysAndValues(keyvals)
	logger := LoggerFromContext(ctx)
	logger.Log(zap.ErrorLevel, msg, fields...)
}

func Error(ctx context.Context, msg string, err error) {
	logger := LoggerFromContext(ctx)
	logger.Log(zap.ErrorLevel, msg, zap.Error(err))
}
