package biz

import (
	"ai-rag-demo/internal/biz/base"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	base.ProviderSet,
)
