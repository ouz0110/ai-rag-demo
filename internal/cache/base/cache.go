package base

import (
	"fmt"

	"ai-rag-demo/internal/conf"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	cfg *conf.Config
	cli *redis.Client
}

const cacheName = "base"

func NewCache(cfg *conf.Config) *Cache {
	cli, err := conf.NewRedisClient(cacheName, cfg)
	if err != nil {
		fmt.Printf("Cache %s init err: %v\n", cacheName, err)
		return nil
	}
	fmt.Printf("Cache %s init success\n", cacheName)
	return &Cache{
		cli: cli,
		cfg: cfg,
	}
}

func (s *Cache) GetCli() *redis.Client {
	return s.cli
}
