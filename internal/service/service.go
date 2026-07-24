package service

import (
	"ai-rag-demo/internal/service/base"
	"ai-rag-demo/internal/service/nocli"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	base.ProviderSet,
	nocli.ProviderSet,
)
