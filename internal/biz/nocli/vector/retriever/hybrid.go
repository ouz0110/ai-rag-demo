package retriever

import (
	"context"
	"sort"
	"sync"

	vectorData "ai-rag-demo/internal/biz/nocli/vector/store"
	"ai-rag-demo/internal/common"
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

// HybridRetriever 双路混合检索融合执行器 (Dense 向量语义 + Sparse/BM25 文本关键词 + RRF 融合算子)
type HybridRetriever struct {
	vectorStore vectorData.VectorStore
	kConstant   float32 // RRF 倒数排名融合常数 (默认 60.0)
}

func NewHybridRetriever(vectorStore vectorData.VectorStore) *HybridRetriever {
	return &HybridRetriever{
		vectorStore: vectorStore,
		kConstant:   60.0,
	}
}

// Retrieve 执行 Concurrent 双路召回 (Dense 向量 + BM25/Text 关键词) 并进行 RRF 倒数排名融合
func (r *HybridRetriever) Retrieve(ctx context.Context, collectionName string, query *vectorData.SearchQuery, queryText string) ([]*ScoredChunk, error) {
	if r.vectorStore == nil {
		return nil, nil
	}

	var denseResults []*vectorData.VectorSearchResult
	var sparseResults []*vectorData.VectorSearchResult
	var denseErr, sparseErr error

	var wg sync.WaitGroup
	wg.Add(2)

	// 路 1: 并发执行 Dense Vector 向量语义检索 (遵守安全协程规范)
	common.RunInGoroutine(ctx, func(gCtx context.Context) {
		defer wg.Done()
		denseResults, denseErr = r.vectorStore.Search(gCtx, collectionName, query)
	})

	// 路 2: 并发执行 Sparse/BM25 文本关键词精确检索 (遵守安全协程规范)
	common.RunInGoroutine(ctx, func(gCtx context.Context) {
		defer wg.Done()
		topK := 10
		tenantID := ""
		if query != nil {
			topK = query.TopK
			tenantID = query.TenantID
		}
		sparseResults, sparseErr = r.vectorStore.SearchText(gCtx, collectionName, tenantID, queryText, topK)
	})

	wg.Wait()

	if denseErr != nil {
		return nil, denseErr
	}
	if sparseErr != nil {
		log.Warnf(ctx, "[HybridRetriever] Sparse text search warning: %v, fallback to dense results", sparseErr)
	}

	// RRF 倒数排名融合算法 (Reciprocal Rank Fusion)
	rrfScores := make(map[string]float32)
	chunkMap := make(map[string]*vectorData.VectorSearchResult)

	for rank, item := range denseResults {
		if item == nil || item.ID == "" {
			continue
		}
		chunkMap[item.ID] = item
		rrfScores[item.ID] += 1.0 / (r.kConstant + float32(rank+1))
	}

	for rank, item := range sparseResults {
		if item == nil || item.ID == "" {
			continue
		}
		if _, ok := chunkMap[item.ID]; !ok {
			chunkMap[item.ID] = item
		}
		rrfScores[item.ID] += 1.0 / (r.kConstant + float32(rank+1))
	}

	type rrfItem struct {
		id    string
		score float32
	}
	rrfList := make([]rrfItem, 0, len(rrfScores))
	for id, score := range rrfScores {
		rrfList = append(rrfList, rrfItem{id: id, score: score})
	}

	sort.Slice(rrfList, func(i, j int) bool {
		return rrfList[i].score > rrfList[j].score
	})

	finalChunks := make([]*ScoredChunk, 0, len(rrfList))
	for _, item := range rrfList {
		raw := chunkMap[item.id]
		parentID := ""
		if raw.Metadata != nil {
			if pid, ok := raw.Metadata["parent_id"].(string); ok {
				parentID = pid
			}
		}

		finalChunks = append(finalChunks, &ScoredChunk{
			ChunkID:  raw.ID,
			DocID:    raw.DocID,
			ParentID: parentID,
			Score:    item.score,
			Content:  raw.Content,
			Metadata: raw.Metadata,
		})
	}

	return finalChunks, nil
}
