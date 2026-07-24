package utils

import (
	"context"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func GenLocalize(lang string) *i18n.Localizer {
	bundle := i18n.NewBundle(language.Chinese)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	switch lang {
	case "zh-hant-MO", "zh-hant-TW", "zh-hant-HK":
		bundle.LoadMessageFile("active.zh-hant.toml")
	default:
		bundle.LoadMessageFile("active.zh.toml")
	}
	return i18n.NewLocalizer(bundle, lang)
}

type localizerKey struct{}

func SetI18n(ctx context.Context, loc *i18n.Localizer) context.Context {
	return context.WithValue(ctx, localizerKey{}, loc)
}

func I18nFromContext(ctx context.Context) *i18n.Localizer {
	return ctx.Value(localizerKey{}).(*i18n.Localizer)
}
