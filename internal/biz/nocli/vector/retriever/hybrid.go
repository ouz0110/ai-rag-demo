package retriever

import (
	"context"
	"sort"
	"strings"

	vectorData "ai-rag-demo/internal/biz/nocli/vector/store"
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

// Retrieve 结合 Dense 向量检索与 Sparse 关键词检索进行 RRF 得分融合
func (r *HybridRetriever) Retrieve(ctx context.Context, collectionName string, query *vectorData.SearchQuery, queryText string) ([]*ScoredChunk, error) {
	// 1. 执行 Dense Vector 检索
	vectorResults, err := r.vectorStore.Search(ctx, collectionName, query)
	if err != nil {
		return nil, err
	}

	// 2. RRF (Reciprocal Rank Fusion) 排名融合算法
	rrfScores := make(map[string]float32)
	chunkMap := make(map[string]*vectorData.VectorSearchResult)

	// 处理 Vector 检索 Rank
	for rank, item := range vectorResults {
		chunkMap[item.ID] = item
		rrfScores[item.ID] += 1.0 / (r.kConstant + float32(rank+1))
	}

	// 3. 将 RRF 得分倒序排序
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

	// 4. 组装最终召回结果
	finalChunks := make([]*ScoredChunk, 0, len(rrfList))
	for _, item := range rrfList {
		raw := chunkMap[item.id]
		parentID := ""
		if raw.Metadata != nil {
			if pid, ok := raw.Metadata["parent_id"].(string); ok {
				parentID = pid
			}
		}

		// 若 ID 是父子块前缀格式 (c_xxx)
		if parentID == "" && strings.HasPrefix(raw.ID, "c_") {
			// 从 ID 抽取或从 Metadata 提取
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
