package i18n

import (
	"context"
	"errors"
	"strings"

	"ai-rag-demo/internal/pkg/log"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var bundle *i18n.Bundle

type noErr struct{}

var NoErr noErr

type contextKey struct{}

func Init(defaulz string) error {
	// golang.org/x/text@v0.12.0/internal/language/compact/tables.go compact.ID
	tags, _, err := language.ParseAcceptLanguage(defaulz)
	if err != nil {
		return err
	}
	bundle = i18n.NewBundle(tags[0])
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	return nil
}

func ParseMessageFileBytes(data []byte, path string) error {
	_, err := bundle.ParseMessageFileBytes(data, path)
	return err
}

func NewLocalizer(langs ...string) *i18n.Localizer {
	return i18n.NewLocalizer(bundle, langs...)
}

func (n noErr) GetMessage(ctx context.Context, messageID string, kvpairsOrTemplateData ...any) string {
	msg, err := GetMessage(ctx, messageID, kvpairsOrTemplateData...)
	if err != nil {
		log.Warnf(ctx, "WARN: error when localize message '%s', error: %v\n", messageID, err)
	}

	return msg
}

func (n noErr) GetDefaultMessage(messageID string, kvpairsOrTemplateData ...any) string {
	msg, err := GetDefaultMessage(messageID, kvpairsOrTemplateData...)
	if err != nil {
		log.Warnf(context.Background(), "WARN: error when localize default message '%s', error: %v\n", messageID, err)
	}

	return msg
}

func GetDefaultMessage(messageID string, kvpairsOrTemplateData ...any) (string, error) {
	ctx := context.WithValue(context.Background(), contextKey{}, NewLocalizer())
	return GetMessage(ctx, messageID, kvpairsOrTemplateData...)
}

func WithLocalizer(ctx context.Context, l *i18n.Localizer) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

func GetMessage(ctx context.Context, messageID string, kvpairsOrTemplateData ...any) (string, error) {
	if len(kvpairsOrTemplateData) > 1 && len(kvpairsOrTemplateData)%2 != 0 {
		return "", errors.New("key value must appear in pairs")
	}
	localizer, ok := ctx.Value(contextKey{}).(*i18n.Localizer)
	if !ok {
		return "", errors.New("i18n localizer is missing from context")
	}

	var templateData any
	if l := len(kvpairsOrTemplateData); l == 1 {
		templateData = kvpairsOrTemplateData[0]
	} else if l > 1 {
		td := make(map[string]interface{}, len(kvpairsOrTemplateData))
		for i := 0; i < len(kvpairsOrTemplateData); i += 2 {
			td[kvpairsOrTemplateData[i].(string)] = kvpairsOrTemplateData[i+1]
		}
		templateData = td
	}

	return localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    strings.ToLower(messageID), // 一律为小写（不区分大小写）
		TemplateData: templateData,
	})
}

func GetErrMessageByArgs(ctx context.Context, reason string, kvpairsOrTemplateData ...any) (string, error) {
	messageId := "error." + strings.ToLower(reason)
	return GetMessage(ctx, messageId, kvpairsOrTemplateData...)
}
