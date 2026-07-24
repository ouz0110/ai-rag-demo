package conf

import (
	"ai-rag-demo/internal/pkg/cache"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(name string, c *Config) (*redis.Client, error) {
	cfg, ok := c.Source.Redis[name]
	if !ok {
		return nil, fmt.Errorf("redis '%s' config missing", name)
	}

	return cache.New(&cache.Config{
		Addrs:        cfg.Addrs,
		Password:     cfg.Password,
		ReadTimeout:  cfg.ReadTimeout.Duration,
		WriteTimeout: cfg.WriteTimeout.Duration,
		DB:           cfg.DB,
	})
}
