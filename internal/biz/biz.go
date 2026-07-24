package biz

import (
	"ai-rag-demo/internal/biz/base"
	"ai-rag-demo/internal/biz/nocli"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	base.ProviderSet,
	nocli.ProviderSet,
)
