package base

import (
	"context"
	"time"

	"ai-rag-demo/internal/pkg/utils"
)

func (c *Cache) SetToken(ctx context.Context, openid, token string) error {
	key := utils.JWTTokenCacheKey(openid)
	return c.cli.Set(ctx, key, token, utils.JWTExpireDays*24*time.Hour).Err()
}

func (c *Cache) CheckToken(ctx context.Context, openid, token string) bool {
	key := utils.JWTTokenCacheKey(openid)
	cached, err := c.cli.Get(ctx, key).Result()
	if err != nil || cached == "" {
		return false
	}
	return cached == token
}
