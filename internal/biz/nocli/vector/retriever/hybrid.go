package retriever

import (
	"context"
	"sort"

	vectorData "ai-rag-demo/internal/biz/nocli/vector/store"
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/pkg/log"
	"sync"
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

// HybridRetriever 混合检索与 RRF 融合执行器
type HybridRetriever struct {
	vectorStore vectorData.VectorStore // 底层向量存储适配器
	kConstant   float32                // RRF 排名平滑常数 (默认 60.0)
}

func NewHybridRetriever(vectorStore vectorData.VectorStore) *HybridRetriever {
	return &HybridRetriever{
		vectorStore: vectorStore,
		kConstant:   60.0,
	}
}

// Retrieve 结合 Dense 向量检索与 Sparse 关键词检索进行并发双路召回与 RRF 得分融合
func (r *HybridRetriever) Retrieve(ctx context.Context, collectionName string, query *vectorData.SearchQuery, queryText string) ([]*ScoredChunk, error) {
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
		log.Warnf(ctx, "Sparse search warning: %v, fallback to dense results", sparseErr)
	}

	// 2. RRF (Reciprocal Rank Fusion) 排名融合算法
	rrfScores := make(map[string]float32)
	chunkMap := make(map[string]*vectorData.VectorSearchResult)

	// 融合路 1: Dense Vector 检索 Rank 得分
	for rank, item := range denseResults {
		if item == nil || item.ID == "" {
			continue
		}
		chunkMap[item.ID] = item
		rrfScores[item.ID] += 1.0 / (r.kConstant + float32(rank+1))
	}

	// 融合路 2: Sparse Text 关键词检索 Rank 得分
	for rank, item := range sparseResults {
		if item == nil || item.ID == "" {
			continue
		}
		if _, ok := chunkMap[item.ID]; !ok {
			chunkMap[item.ID] = item
		}
		rrfScores[item.ID] += 1.0 / (r.kConstant + float32(rank+1))
	}

	// 3. 将 RRF 综合得分倒序排序
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

	// 4. 组装双路融合后的最终召回结果
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
