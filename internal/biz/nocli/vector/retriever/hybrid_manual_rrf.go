package retriever

import (
	"context"
	"sort"
	"sync"

	vectorData "ai-rag-demo/internal/biz/nocli/vector/store"
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/pkg/log"
)

// LegacyManualHybridRetriever [学习与对比参考资料]
// 早期在向量数据库未原生理融合时，由 Go 客户端并发发起双路检索并在内存中执行 RRF (Reciprocal Rank Fusion) 算子的历史实现。
type LegacyManualHybridRetriever struct {
	vectorStore vectorData.VectorStore
	kConstant   float32
}

func NewLegacyManualHybridRetriever(vectorStore vectorData.VectorStore) *LegacyManualHybridRetriever {
	return &LegacyManualHybridRetriever{
		vectorStore: vectorStore,
		kConstant:   60.0,
	}
}

// Retrieve Concurrent 双路召回 + 客户端内存 RRF 排序
func (r *LegacyManualHybridRetriever) Retrieve(ctx context.Context, collectionName string, query *vectorData.SearchQuery, queryText string) ([]*ScoredChunk, error) {
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

	// 路 2: 并发执行 Sparse Text 文本关键词精确匹配检索 (遵守安全协程规范)
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
		log.Warnf(ctx, "[Legacy RRF] Sparse search warning: %v, fallback to dense results", sparseErr)
	}

	// RRF 倒数排名融合算法
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
