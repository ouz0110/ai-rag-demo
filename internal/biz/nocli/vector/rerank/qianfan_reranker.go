package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

// QianfanReranker 基于百度千帆 Rerank API 的真实重排序策略实现
type QianfanReranker struct {
	client  *http.Client
	apiKey  string
	baseURL string
	model   string
}

type qianfanRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type qianfanRerankResult struct {
	Document       string  `json:"document"`
	RelevanceScore float64 `json:"relevance_score"`
	Index          int     `json:"index"`
}

type qianfanRerankResponse struct {
	ID      string                `json:"id"`
	Object  string                `json:"object"`
	Created int64                 `json:"created"`
	Model   string                `json:"model"`
	Results []qianfanRerankResult `json:"results"`
	Usage   struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func NewQianfanReranker(cfg *conf.Config) *QianfanReranker {
	apiKey := ""
	baseURL := "https://qianfan.baidubce.com"
	modelName := "bce-reranker-base"

	if cfg != nil {
		if cfg.Source.RAG != nil && cfg.Source.RAG.Rerank != nil {
			apiKey = cfg.Source.RAG.Rerank.APIKey
			if cfg.Source.RAG.Rerank.BaseURL != "" {
				baseURL = cfg.Source.RAG.Rerank.BaseURL
			}
			if cfg.Source.RAG.Rerank.Model != "" {
				modelName = cfg.Source.RAG.Rerank.Model
			}
		}
	}

	timeout := 30 * time.Second
	if cfg != nil && cfg.Source.RAG != nil && cfg.Source.RAG.Rerank != nil && cfg.Source.RAG.Rerank.Timeout > 0 {
		timeout = time.Duration(cfg.Source.RAG.Rerank.Timeout) * time.Millisecond
	}

	return &QianfanReranker{
		client:  &http.Client{Timeout: timeout},
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   modelName,
	}
}

func (r *QianfanReranker) Rerank(ctx context.Context, query string, candidates []*RerankCandidate) ([]*RerankCandidate, openai.Usage, error) {
	if len(candidates) <= 1 {
		return candidates, openai.Usage{}, nil
	}

	documents := make([]string, len(candidates))
	for i, c := range candidates {
		documents[i] = c.Content
	}

	reqBody := qianfanRerankRequest{
		Model:     r.model,
		Query:     query,
		Documents: documents,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, openai.Usage{}, fmt.Errorf("marshal qianfan rerank request: %w", err)
	}

	endpoint := r.baseURL
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, openai.Usage{}, fmt.Errorf("build qianfan rerank request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, openai.Usage{}, fmt.Errorf("qianfan rerank request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, openai.Usage{}, fmt.Errorf("qianfan rerank API returned status %d, err: %s", resp.StatusCode, string(body))
	}

	var qfResp qianfanRerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&qfResp); err != nil {
		return nil, openai.Usage{}, fmt.Errorf("decode qianfan rerank response: %w", err)
	}

	if len(qfResp.Results) == 0 {
		return candidates, openai.Usage{
			PromptTokens:     qfResp.Usage.PromptTokens,
			CompletionTokens: 0,
			TotalTokens:      qfResp.Usage.TotalTokens,
		}, nil
	}

	usage := openai.Usage{
		PromptTokens:     qfResp.Usage.PromptTokens,
		CompletionTokens: 0,
		TotalTokens:      qfResp.Usage.TotalTokens,
	}

	results := make([]*RerankCandidate, len(candidates))
	copy(results, candidates)

	for _, r := range qfResp.Results {
		if r.Index >= 0 && r.Index < len(results) {
			results[r.Index].Score = float32(r.RelevanceScore)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	log.Infof(ctx, "Qianfan rerank complete: model=%s, candidates=%d, prompt_tokens=%d, total_tokens=%d",
		r.model, len(candidates), usage.PromptTokens, usage.TotalTokens)

	return results, usage, nil
}
