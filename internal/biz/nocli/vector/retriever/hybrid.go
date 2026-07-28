package retriever

import (
	"context"

	vectorData "ai-rag-demo/internal/biz/nocli/vector/store"
	"ai-rag-demo/internal/pkg/log"
)

// ScoredChunk 混合检索融合得分后的切片实体
type ScoredChunk struct {
	ChunkID  string                 `json:"chunk_id"`  // 切片唯一 ID
	DocID    string                 `json:"doc_id"`    // 所属主文档 ID
	ParentID string                 `json:"parent_id"` // 父切片 ID (用于回查粗粒度上下文)
	Score    float32                `json:"score"`     // 融合排序得分 (RRF Score)
	Content  string                 `json:"content"`   // 文本切片内容
	Metadata map[string]interface{} `json:"metadata"`  // 关联标量元数据
}

// HybridRetriever 主流原生的 Milvus 2.4+ 服务端 HybridSearch 检索融合执行器
type HybridRetriever struct {
	vectorStore vectorData.VectorStore // 底层向量存储适配器
	legacyRRF   *LegacyManualHybridRetriever // 历史客户端手写双路 RRF (做兜底与对比)
}

func NewHybridRetriever(vectorStore vectorData.VectorStore) *HybridRetriever {
	return &HybridRetriever{
		vectorStore: vectorStore,
		legacyRRF:   NewLegacyManualHybridRetriever(vectorStore),
	}
}

// Retrieve 主流解法：直接调用向量数据库服务端的原生 HybridSearch 接口完成多路召回与 RRF 融合
func (r *HybridRetriever) Retrieve(ctx context.Context, collectionName string, query *vectorData.SearchQuery, queryText string) ([]*ScoredChunk, error) {
	if r.vectorStore == nil {
		return nil, nil
	}

	// 1. 优先使用 Milvus 2.4+ 原生服务端 HybridSearch
	results, err := r.vectorStore.HybridSearch(ctx, collectionName, query, queryText)
	if err != nil {
		log.Warnf(ctx, "[HybridRetriever] Native HybridSearch error: %v, falling back to Legacy Concurrent RRF", err)
		// 降级使用历史手写并发 RRF 检索引擎
		return r.legacyRRF.Retrieve(ctx, collectionName, query, queryText)
	}

	if len(results) == 0 {
		return nil, nil
	}

	// 2. 转化为 RAG 门面层 ScoredChunk
	scoredChunks := make([]*ScoredChunk, 0, len(results))
	for _, res := range results {
		parentID := ""
		if res.Metadata != nil {
			if pid, ok := res.Metadata["parent_id"].(string); ok {
				parentID = pid
			}
		}

		scoredChunks = append(scoredChunks, &ScoredChunk{
			ChunkID:  res.ID,
			DocID:    res.DocID,
			ParentID: parentID,
			Score:    res.Score,
			Content:  res.Content,
			Metadata: res.Metadata,
		})
	}

	return scoredChunks, nil
}
