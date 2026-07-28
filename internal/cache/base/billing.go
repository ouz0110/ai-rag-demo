package base

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type BillingCache struct {
	rdb redis.UniversalClient
}

func NewBillingCache(rdb redis.UniversalClient) *BillingCache {
	return &BillingCache{rdb: rdb}
}

func (c *BillingCache) balanceKey(userID string) string {
	return fmt.Sprintf("billing:balance:%s", userID)
}

// PreCheckBalance Lua 脚本预校验余额
func (c *BillingCache) PreCheckBalance(ctx context.Context, userID string, minRequiredCost float64) (bool, error) {
	if c.rdb == nil {
		return true, nil
	}

	key := c.balanceKey(userID)
	luaScript := `
		local val = redis.call('GET', KEYS[1])
		if not val then
			return -1
		end
		local balance = tonumber(val)
		if balance >= tonumber(ARGV[1]) then
			return 1
		else
			return 0
		end
	`

	res, err := c.rdb.Eval(ctx, luaScript, []string{key}, minRequiredCost).Result()
	if err != nil {
		return true, nil
	}

	code, _ := res.(int64)
	if code == 0 {
		return false, nil
	}
	return true, nil
}

// SetBalanceCache 设置用户余额缓存
func (c *BillingCache) SetBalanceCache(ctx context.Context, userID string, balance float64) error {
	if c.rdb == nil {
		return nil
	}
	key := c.balanceKey(userID)
	return c.rdb.Set(ctx, key, strconv.FormatFloat(balance, 'f', 6, 64), 24*time.Hour).Err()
}

// DeductBalanceCache 缓存额度更新
func (c *BillingCache) DeductBalanceCache(ctx context.Context, userID string, cost float64) error {
	if c.rdb == nil {
		return nil
	}
	key := c.balanceKey(userID)
	luaScript := `
		local val = redis.call('GET', KEYS[1])
		if val then
			local new_bal = tonumber(val) - tonumber(ARGV[1])
			redis.call('SET', KEYS[1], tostring(new_bal), 'EX', 86400)
			return new_bal
		end
		return nil
	`
	_, err := c.rdb.Eval(ctx, luaScript, []string{key}, cost).Result()
	return err
}
