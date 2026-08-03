package rerank

import (
	"context"
	"strings"

	"ai-rag-demo/internal/conf"

	openai "github.com/sashabaranov/go-openai"
)

// NoOpReranker 不做任何打分的空落地方案 (作为降级或未开启时的默认策略)
type NoOpReranker struct{}

func (n *NoOpReranker) Rerank(ctx context.Context, query string, candidates []*RerankCandidate) ([]*RerankCandidate, openai.Usage, error) {
	return candidates, openai.Usage{}, nil
}

// NewReranker 根据配置文件初始化对应 Driver 的 Reranker (llm / qianfan / NoOp)
func NewReranker(cfg *conf.Config) Reranker {
	if cfg == nil || cfg.Source.RAG == nil || cfg.Source.RAG.Rerank == nil {
		return &NoOpReranker{}
	}

	rCfg := cfg.Source.RAG.Rerank
	if !rCfg.Enable {
		return &NoOpReranker{}
	}

	driver := strings.ToLower(rCfg.Driver)
	switch driver {
	case "llm":
		return NewLLMReranker(cfg)
	case "qianfan":
		return NewQianfanReranker(cfg)
	default:
		return &NoOpReranker{}
	}
}
