package biz

import (
	"ai-rag-demo/internal/biz/base"
	"ai-rag-demo/internal/biz/common"
	"ai-rag-demo/internal/biz/nocli"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	common.ProviderSet,
	base.ProviderSet,
	nocli.ProviderSet,
)
