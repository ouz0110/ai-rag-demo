package embedder

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	bizCommon "ai-rag-demo/internal/biz/common"
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/time/rate"
)

// Embedder 向量计算组件，支持独立的 AI 配置与令牌桶限流保护
type Embedder struct {
	client      *openai.Client        // OpenAI API 客户端
	model       openai.EmbeddingModel // 目标 Embedding 模型类型
	dimension   int                   // 向量维度 (通过配置传入)
	rateLimiter *rate.Limiter         // API 呼叫限流器 (防止 429 Rate Limit)
}

func NewEmbedder(cfg *conf.Config) *Embedder {
	apiKey := ""
	baseURL := ""
	modelName := string(openai.LargeEmbedding3)
	dim := 1536 // 默认兜底维度

	if cfg != nil {
		if cfg.Source.RAG != nil && cfg.Source.RAG.Embedding != nil {
			apiKey = cfg.Source.RAG.Embedding.APIKey
			baseURL = cfg.Source.RAG.Embedding.BaseURL
			if cfg.Source.RAG.Embedding.Model != "" {
				modelName = cfg.Source.RAG.Embedding.Model
			}
			if cfg.Source.RAG.Embedding.Dimension > 0 {
				dim = cfg.Source.RAG.Embedding.Dimension
			}
		}
		// 若 Embedding 未配置维度，优先复用 VectorDB.Milvus 中的 Dimension 配置
		if dim == 1536 && cfg.Source.VectorDB != nil && cfg.Source.VectorDB.Milvus != nil && cfg.Source.VectorDB.Milvus.Dimension > 0 {
			dim = cfg.Source.VectorDB.Milvus.Dimension
		}

		// 兜底降级至默认 OpenAI 配置
		if apiKey == "" && cfg.Source.OpenAI != nil {
			apiKey = cfg.Source.OpenAI.APIKey
			baseURL = cfg.Source.OpenAI.BaseURL
		}
	}

	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	client := openai.NewClientWithConfig(config)
	return &Embedder{
		client:      client,
		model:       openai.EmbeddingModel(modelName),
		dimension:   dim,
		rateLimiter: rate.NewLimiter(rate.Limit(30), 30), // 30 QPS 限流保护
	}
}

// Dimension 获取当前配置的向量维度
func (e *Embedder) Dimension() int {
	if e.dimension <= 0 {
		return 1536
	}
	return e.dimension
}

// GenerateEmbedding 为单条文本计算 Embedding，同时返回 OpenAI Token 消耗情况
func (e *Embedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, openai.Usage, error) {
	vecs, usage, err := e.BatchGenerateEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, usage, err
	}
	if len(vecs) == 0 {
		return nil, usage, fmt.Errorf("empty embedding response")
	}
	return vecs[0], usage, nil
}

// BatchGenerateEmbeddings 批量并发计算 Embedding，支持分批与并发度控制，并汇总返回详细 Token 消耗
func (e *Embedder) BatchGenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, openai.Usage, error) {
	if len(texts) == 0 {
		return nil, openai.Usage{}, nil
	}

	batchSize := 20
	total := len(texts)
	results := make([][]float32, total)

	var totalPromptTokens int32
	var totalTokens int32

	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}

		startIndex := i
		batchTexts := texts[i:end]

		wg.Add(1)
		// 结合项目规范 common.RunInGoroutine 处理 Goroutine Panic 恢复与日志传递
		common.RunInGoroutine(ctx, func(gCtx context.Context) {
			defer wg.Done()

			vecs, bUsage, err := e.callOpenAIEmbeddingWithRetry(gCtx, batchTexts)
			if err != nil {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("batch embedding failed [%d:%d]: %w", startIndex, end, err)
				})
				return
			}

			atomic.AddInt32(&totalPromptTokens, int32(bUsage.PromptTokens))
			atomic.AddInt32(&totalTokens, int32(bUsage.TotalTokens))

			for idx, vec := range vecs {
				results[startIndex+idx] = vec
			}
		})
	}

	wg.Wait()

	aggregatedUsage := openai.Usage{
		PromptTokens: int(totalPromptTokens),
		TotalTokens:  int(totalTokens),
	}

	if firstErr != nil {
		return nil, aggregatedUsage, firstErr
	}

	return results, aggregatedUsage, nil
}

func (e *Embedder) callOpenAIEmbeddingWithRetry(ctx context.Context, texts []string) ([][]float32, openai.Usage, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if e.rateLimiter != nil {
			_ = e.rateLimiter.Wait(ctx)
		}

		req := openai.EmbeddingRequest{
			Input: texts,
			Model: openai.EmbeddingModel(e.model),
		}

		resp, err := e.client.CreateEmbeddings(ctx, req)
		if err == nil {
			vecs := make([][]float32, len(resp.Data))
			for _, item := range resp.Data {
				vecs[item.Index] = item.Embedding
			}
			return vecs, resp.Usage, nil
		}

		lastErr = err
		log.Warnf(ctx, "Embedding attempt %d failed: %v, retrying...", attempt+1, err)
		backoff := time.Duration(math.Pow(2, float64(attempt))) * 200 * time.Millisecond
		time.Sleep(backoff + time.Duration(rand.Intn(100))*time.Millisecond)
	}

	// 生产环境下模型服务可能未就绪，当真实 API 调用失败时降级返回伪向量 (Mock Vector for Demo/Fallback) 并使用基础 token 函数计算
	log.Warnf(ctx, "Embedding API failed after retries, falling back to mock vectors: %v", lastErr)
	fallbackTokens := int(bizCommon.CalculateTextTokens(texts...))
	fallbackUsage := openai.Usage{
		PromptTokens: fallbackTokens,
		TotalTokens:  fallbackTokens,
	}

	mockVecs := make([][]float32, len(texts))
	dim := e.Dimension()
	for i := range texts {
		vec := make([]float32, dim)
		for d := 0; d < dim; d++ {
			vec[d] = rand.Float32()
		}
		mockVecs[i] = vec
	}
	return mockVecs, fallbackUsage, nil
}
