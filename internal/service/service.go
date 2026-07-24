package service

import (
	"ai-rag-demo/internal/service/base"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	base.ProviderSet,
)
