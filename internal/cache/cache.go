package cache

import (
	base "ai-rag-demo/internal/cache/base"
	"ai-rag-demo/internal/conf"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAllCache,
)

type Cache struct {
	cfg  *conf.Config
	Base *base.Cache
}

func NewAllCache(c *conf.Config) *Cache {
	return &Cache{
		cfg:  c,
		Base: base.NewCache(c),
	}
}
