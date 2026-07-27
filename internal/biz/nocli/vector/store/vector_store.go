package store

import (
	"context"
)

// VectorDocument 写入向量数据库的文档块结构
type VectorDocument struct {
	ID       string                 `json:"id"`        // 块唯一 ID (Chunk ID)
	DocID    string                 `json:"doc_id"`    // 所属主文档 ID
	TenantID string                 `json:"tenant_id"` // 租户 ID
	Vector   []float32              `json:"vector"`    // Embedding 向量数据
	Content  string                 `json:"content"`   // 文本块内容
	Metadata map[string]interface{} `json:"metadata"`  // 标量元数据
}

// VectorSearchResult 向量数据库查询返回的匹配结果
type VectorSearchResult struct {
	ID       string                 `json:"id"`
	DocID    string                 `json:"doc_id"`
	Score    float32                `json:"score"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
}

// SearchQuery 向量检索查询入参
type SearchQuery struct {
	TenantID   string    `json:"tenant_id"`   // 租户 ID (Partition Key)
	KBID       string    `json:"kb_id"`       // 知识库 ID
	Vector     []float32 `json:"vector"`      // 查询向量
	TopK       int       `json:"top_k"`       // 召回数量
	MinScore   float32   `json:"min_score"`   // 最低相似度阈值
	OnlyActive bool      `json:"only_active"` // 是否仅检索生效切片 (过滤 is_active == 1)
	Category   string    `json:"category"`    // 业务分类过滤
}

// VectorStore 向量数据库驱动高层抽象接口 (Vector DB Agnostic)
type VectorStore interface {
	CreateCollection(ctx context.Context, collectionName string, dim int) error
	HasCollection(ctx context.Context, collectionName string) (bool, error)
	Upsert(ctx context.Context, collectionName string, docs []*VectorDocument) error
	BatchDelete(ctx context.Context, collectionName string, ids []string) error
	DeleteByDocID(ctx context.Context, collectionName string, tenantID, docID string) error
	Search(ctx context.Context, collectionName string, query *SearchQuery) ([]*VectorSearchResult, error)
}
