package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm/logger"
)

type Cache struct {
	cli *redis.Client
}

type Config struct {
	Password     string
	Addrs        string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Logger       logger.Interface
	DB            int
}

func New(cfg *Config) (*redis.Client, error) {
	cli := redis.NewClient(&redis.Options{
		Addr:         cfg.Addrs,
		Password:     cfg.Password,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		DB:           cfg.DB,
	})
	if _, err := cli.Ping(context.Background()).Result(); err != nil {
		return nil, err
	}

	return cli, nil
}
