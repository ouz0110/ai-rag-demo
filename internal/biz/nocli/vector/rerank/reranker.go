package rerank

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
)

// RerankCandidate 待精排的切片候选实体
type RerankCandidate struct {
	ID       string                 `json:"id"`        // 切片唯一 ID
	DocID    string                 `json:"doc_id"`    // 所属文档 ID
	ParentID string                 `json:"parent_id"` // 父切片 ID
	Content  string                 `json:"content"`   // 文本内容
	Score    float32                `json:"score"`     // 粗筛得分
	Metadata map[string]interface{} `json:"metadata"`  // 元数据
}

// Reranker 重排序组件策略接口 (Strategy Pattern)
type Reranker interface {
	// Rerank 对候选列表按 Query 相关度进行深度二次打分与重新排序，同时返回 AI Usage
	Rerank(ctx context.Context, query string, candidates []*RerankCandidate) ([]*RerankCandidate, openai.Usage, error)
}
