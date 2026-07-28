package rerank

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

// LLMReranker 基于大语言模型的 Rerank 打分与重排策略实现
type LLMReranker struct {
	client *openai.Client // OpenAI API 客户端句柄
	model  string         // 使用的模型名称 (如 gpt-4o-mini 或 qwen)
}

type llmScoreItem struct {
	ID    string  `json:"id"`    // 切片 ID
	Score float32 `json:"score"` // 相似度打分 (0.0 - 1.0)
}

func NewLLMReranker(cfg *conf.Config) *LLMReranker {
	apiKey := ""
	baseURL := ""
	modelName := "gpt-4o-mini"

	if cfg != nil {
		if cfg.Source.RAG != nil && cfg.Source.RAG.Rerank != nil {
			apiKey = cfg.Source.RAG.Rerank.APIKey
			baseURL = cfg.Source.RAG.Rerank.BaseURL
			if cfg.Source.RAG.Rerank.Model != "" {
				modelName = cfg.Source.RAG.Rerank.Model
			}
		}
		// 兜底降级使用全局 OpenAI 配置
		if apiKey == "" && cfg.Source.OpenAI != nil {
			apiKey = cfg.Source.OpenAI.APIKey
			baseURL = cfg.Source.OpenAI.BaseURL
			if cfg.Source.OpenAI.Model != "" && modelName == "" {
				modelName = cfg.Source.OpenAI.Model
			}
		}
	}

	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	client := openai.NewClientWithConfig(config)
	return &LLMReranker{
		client: client,
		model:  modelName,
	}
}

func (r *LLMReranker) Rerank(ctx context.Context, query string, candidates []*RerankCandidate) ([]*RerankCandidate, openai.Usage, error) {
	if len(candidates) <= 1 {
		return candidates, openai.Usage{}, nil
	}

	// 1. 组装 LLM 打分 Prompt
	var docBuilder strings.Builder
	for idx, c := range candidates {
		docBuilder.WriteString(fmt.Sprintf("[%d] ID: %s\n内容: %s\n\n", idx+1, c.ID, c.Content))
	}

	prompt := fmt.Sprintf(`用户检索问题: "%s"

请对以下文档片段与问题的相关性进行打分(得分范围 0.0 至 1.0，1.0 表示极度匹配，0.0 表示完全无关)。
必须且仅返回 JSON 数组格式，禁止输出任何多余说明文本：
[
  {"id": "切片ID", "score": 0.95}
]

待打分文档列表:
%s`, query, docBuilder.String())

	req := openai.ChatCompletionRequest{
		Model: r.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "你是一个专业的搜索相关性打分专家。必须以极度严谨的标尺评估文档与问题的匹配度，并仅返回 JSON 格式结果。",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.1,
	}

	resp, err := r.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, openai.Usage{}, fmt.Errorf("llm rerank request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, resp.Usage, fmt.Errorf("empty llm rerank choices")
	}

	rawText := strings.TrimSpace(resp.Choices[0].Message.Content)
	// 清理可能的 markdown 代码块标记
	rawText = strings.TrimPrefix(rawText, "```json")
	rawText = strings.TrimPrefix(rawText, "```")
	rawText = strings.TrimSuffix(rawText, "```")
	rawText = strings.TrimSpace(rawText)

	var scoreItems []llmScoreItem
	if err := json.Unmarshal([]byte(rawText), &scoreItems); err != nil {
		log.Warnf(ctx, "Parse LLM rerank JSON response error: %v, rawText: %s", err, rawText)
		return candidates, resp.Usage, nil // 解析失败降级返回原列表与真实 usage
	}

	scoreMap := make(map[string]float32)
	for _, item := range scoreItems {
		scoreMap[item.ID] = item.Score
	}

	// 2. 刷新候选列表的分值
	results := make([]*RerankCandidate, len(candidates))
	copy(results, candidates)

	for _, res := range results {
		if score, ok := scoreMap[res.ID]; ok {
			res.Score = score
		}
	}

	// 3. 按最新相关性得分倒序重排
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, resp.Usage, nil
}
