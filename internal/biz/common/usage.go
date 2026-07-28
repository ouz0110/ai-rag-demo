package common

import (
	"context"
	"errors"
	"math"
	"sync"

	"ai-rag-demo/internal/cache"
	cacheBase "ai-rag-demo/internal/cache/base"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/database"

	openai "github.com/sashabaranov/go-openai"
)

// CalculateTextTokens 基础 token 计算函数：根据文本估算 Token 数量 (用于离线估算或 API 失败时的兜底)
func CalculateTextTokens(texts ...string) int32 {
	var total int32
	for _, text := range texts {
		total += int32(len([]rune(text)))
	}
	return total
}

// ExtractOpenAIUsage 从 OpenAI API 返回的 Usage 对象提取 Token 消耗
func ExtractOpenAIUsage(usage openai.Usage) (promptTokens, completionTokens, totalTokens int32) {
	return int32(usage.PromptTokens), int32(usage.CompletionTokens), int32(usage.TotalTokens)
}

// UsageRecorder 统一的 AI 扣费与 Token 消耗记录器，生成独立 Struct 方便跨 Biz 模块共享调用
type UsageRecorder struct {
	repo         *dataBase.BillingRepo
	cache        *cacheBase.BillingCache
	db           *database.DB
	pricingMap   map[string]*conf.ModelPricingConfig
	pricingMutex sync.RWMutex
}

func NewUsageRecorder(
	allDb *data.DB,
	cache *cache.Cache,
	cfg *conf.Config,
) *UsageRecorder {
	var repo *dataBase.BillingRepo
	var rawDB *database.DB
	if allDb != nil && allDb.Base != nil {
		repo = &allDb.Base.BillingRepo
		rawDB = allDb.Base.DB
	}

	var billingCache *cacheBase.BillingCache
	if cache != nil && cache.Base != nil {
		billingCache = cache.Base.Billing
	}

	recorder := &UsageRecorder{
		repo:       repo,
		cache:      billingCache,
		db:         rawDB,
		pricingMap: make(map[string]*conf.ModelPricingConfig),
	}

	if cfg != nil && cfg.Source.RAG != nil && cfg.Source.RAG.Billing != nil {
		for model, p := range cfg.Source.RAG.Billing.DefaultPrices {
			recorder.pricingMap[model] = p
		}
	}
	return recorder
}

// CalculateCost 根据计费规则计算单次 AI 调用费用 (Token 不足 1k 按照 1k 计算)
func (u *UsageRecorder) CalculateCost(modelName string, promptTokens, completionTokens, docCount int32) float64 {
	u.pricingMutex.RLock()
	price, exists := u.pricingMap[modelName]
	u.pricingMutex.RUnlock()

	if !exists || price == nil {
		price = &conf.ModelPricingConfig{
			InputUnitPrice:  0.001,
			OutputUnitPrice: 0.002,
			UnitSize:        1000,
		}
	}

	unitSize := price.UnitSize
	if unitSize <= 0 {
		unitSize = 1000
	}

	var totalCost float64

	if promptTokens > 0 {
		units := math.Ceil(float64(promptTokens) / float64(unitSize))
		totalCost += units * price.InputUnitPrice
	}

	if completionTokens > 0 {
		units := math.Ceil(float64(completionTokens) / float64(unitSize))
		outPrice := price.OutputUnitPrice
		if outPrice <= 0 {
			outPrice = price.InputUnitPrice
		}
		totalCost += units * outPrice
	}

	if docCount > 0 && promptTokens == 0 {
		units := math.Ceil(float64(docCount) / float64(unitSize))
		totalCost += units * price.InputUnitPrice
	}

	return totalCost
}

// PreCheckUserBalance 发起 AI 请求前校验余额
func (u *UsageRecorder) PreCheckUserBalance(ctx context.Context, userID string, minRequiredCost float64) error {
	if u.cache != nil {
		ok, err := u.cache.PreCheckBalance(ctx, userID, minRequiredCost)
		if err == nil && !ok {
			return errors.New("insufficient balance, please recharge")
		}
	}

	if u.repo == nil {
		return nil
	}

	userBal, err := u.repo.GetUserBalance(ctx, userID)
	if err != nil {
		return err
	}

	if u.cache != nil {
		_ = u.cache.SetBalanceCache(ctx, userID, userBal.Balance)
	}

	if userBal.Balance < minRequiredCost {
		return errors.New("insufficient balance, please recharge")
	}
	return nil
}

// RecordAndDeductUsage 记录消费明细并执行数据库事务扣费 (Phase 1 & Phase 2 完整扣费落盘)
func (u *UsageRecorder) RecordAndDeductUsage(ctx context.Context, usage *dataBase.BillingUsageLogModel) (float64, error) {
	if usage == nil {
		return 0, nil
	}

	if usage.UserID == "" {
		usage.UserID = "default_user"
	}

	cost := u.CalculateCost(usage.ModelName, usage.PromptTokens, usage.CompletionTokens, usage.DocCount)
	usage.TotalCost = cost
	usage.Status = 1

	if u.repo != nil && usage.RequestID != "" {
		processed, err := u.repo.IsRequestProcessed(ctx, usage.RequestID)
		if err == nil && processed {
			return cost, nil
		}
	}

	if u.db != nil && u.repo != nil {
		err := u.db.InTransaction(ctx, func(txCtx context.Context) error {
			if err := u.repo.CreateUsageLog(txCtx, usage); err != nil {
				return err
			}
			if cost > 0 {
				if err := u.repo.DeductUserBalance(txCtx, usage.UserID, cost); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	} else if u.repo != nil {
		_ = u.repo.CreateUsageLog(ctx, usage)
		if cost > 0 {
			_ = u.repo.DeductUserBalance(ctx, usage.UserID, cost)
		}
	}

	if u.cache != nil {
		_ = u.cache.DeductBalanceCache(ctx, usage.UserID, cost)
	}

	return cost, nil
}

// RecordOpenAIUsage 基于 OpenAI Usage 对象的便捷扣费记录封装方法
func (u *UsageRecorder) RecordOpenAIUsage(
	ctx context.Context,
	reqID, userID string,
	serviceType dataBase.ServiceType,
	provider, modelName string,
	usage openai.Usage,
	docCount int32,
) (float64, error) {
	promptTokens, completionTokens, totalTokens := ExtractOpenAIUsage(usage)
	logModel := &dataBase.BillingUsageLogModel{
		RequestID:        reqID,
		UserID:           userID,
		ServiceType:      serviceType,
		Provider:         provider,
		ModelName:        modelName,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		DocCount:         docCount,
	}
	return u.RecordAndDeductUsage(ctx, logModel)
}

// GetPricingMap 获取当前模型价格配置
func (u *UsageRecorder) GetPricingMap() map[string]*conf.ModelPricingConfig {
	u.pricingMutex.RLock()
	defer u.pricingMutex.RUnlock()
	res := make(map[string]*conf.ModelPricingConfig)
	for k, v := range u.pricingMap {
		res[k] = v
	}
	return res
}
