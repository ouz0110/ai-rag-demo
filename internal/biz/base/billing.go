package base

import (
	"context"

	bizCommon "ai-rag-demo/internal/biz/common"
	"ai-rag-demo/internal/cache"
	cacheBase "ai-rag-demo/internal/cache/base"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data"
	dataBase "ai-rag-demo/internal/data/base"
)

type BillingBiz struct {
	*bizCommon.UsageRecorder
	repo  *dataBase.BillingRepo
	cache *cacheBase.BillingCache
}

func NewBillingBiz(
	allDb *data.DB,
	cache *cache.Cache,
	cfg *conf.Config,
) *BillingBiz {
	var repo *dataBase.BillingRepo
	if allDb != nil && allDb.Base != nil {
		repo = &allDb.Base.BillingRepo
	}

	var billingCache *cacheBase.BillingCache
	if cache != nil && cache.Base != nil {
		billingCache = cache.Base.Billing
	}

	return &BillingBiz{
		UsageRecorder: bizCommon.NewUsageRecorder(allDb, cache, cfg),
		repo:          repo,
		cache:         billingCache,
	}
}

// GetUserBalance 获取指定用户余额
func (b *BillingBiz) GetUserBalance(ctx context.Context, userID string) (*dataBase.UserBalanceModel, error) {
	if b.repo == nil {
		return &dataBase.UserBalanceModel{UserID: userID}, nil
	}
	bal, err := b.repo.GetUserBalance(ctx, userID)
	if err != nil {
		return nil, err
	}
	if b.cache != nil {
		_ = b.cache.SetBalanceCache(ctx, userID, bal.Balance)
	}
	return bal, nil
}

// RechargeUserBalance 用户充值
func (b *BillingBiz) RechargeUserBalance(ctx context.Context, userID string, amount float64) (*dataBase.UserBalanceModel, error) {
	if b.repo == nil {
		return &dataBase.UserBalanceModel{UserID: userID, Balance: amount}, nil
	}
	bal, err := b.repo.RechargeUserBalance(ctx, userID, amount)
	if err != nil {
		return nil, err
	}
	if b.cache != nil {
		_ = b.cache.SetBalanceCache(ctx, userID, bal.Balance)
	}
	return bal, nil
}

// ListUsageLogs 分页获取流水
func (b *BillingBiz) ListUsageLogs(ctx context.Context, userID string, limit, offset int) ([]*dataBase.BillingUsageLogModel, int64, error) {
	if b.repo == nil {
		return nil, 0, nil
	}
	return b.repo.ListUsageLogs(ctx, userID, limit, offset)
}
