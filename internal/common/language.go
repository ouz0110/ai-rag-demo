package common

import "context"

const (
	LangCN = "zh-CN"
	LangEN = "en-US"
)

type languageCtxKey struct{}

func WithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, languageCtxKey{}, lang)
}

func LanguageFromContext(ctx context.Context) string {
	if lang, ok := ctx.Value(languageCtxKey{}).(string); ok {
		return lang
	}
	return LangCN
}
