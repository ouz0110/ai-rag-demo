package rag

import (
	"context"
	"time"

	"ai-rag-demo/internal/pkg/database"
)

// KnowledgeChunkModel 知识库文档切片表结构模型 (用于存储 Parent 粗粒度上下文及切片映射)
type KnowledgeChunkModel struct {
	ID           int64     `gorm:"primaryKey;autoIncrement;comment:主键ID"`                                    // 主键ID
	TenantID     string    `gorm:"type:varchar(64);not null;index:idx_tenant_chunk;comment:租户ID"`              // 租户ID
	DocID        string    `gorm:"type:varchar(64);not null;index;comment:所属文档UUID"`                           // 所属文档UUID
	ChunkID      string    `gorm:"type:varchar(64);not null;uniqueIndex;comment:切片UUID(对应向量库ID)"`              // 切片UUID
	ParentID     string    `gorm:"type:varchar(64);default:'';index;comment:父块UUID(为空表示自身为父块)"`                // 父块UUID
	H1           string    `gorm:"type:varchar(255);default:'';comment:一级标题"`                               // 一级标题
	H2           string    `gorm:"type:varchar(255);default:'';comment:二级标题"`                               // 二级标题
	H3           string    `gorm:"type:varchar(255);default:'';comment:三级标题"`                               // 三级标题
	StartLine    int32     `gorm:"type:int;default:0;comment:起始行号"`                                          // 起始行号
	EndLine      int32     `gorm:"type:int;default:0;comment:结束行号"`                                          // 结束行号
	HasTable     int32     `gorm:"type:tinyint;default:0;comment:是否包含表格(0:否,1:是)"`                           // 是否包含表格
	HasCode      int32     `gorm:"type:tinyint;default:0;comment:是否包含代码块(0:否,1:是)"`                         // 是否包含代码块
	ChunkIndex   int32     `gorm:"type:int;not null;comment:切片序号"`                                           // 切片序号
	ChunkHash    string    `gorm:"type:varchar(64);default:'';comment:切片内容SHA256哈希(用于增量Diff)"`               // 切片内容SHA256哈希
	ChunkType    string    `gorm:"type:varchar(32);not null;default:'text';comment:切片类型(parent,text,table,code)"` // 切片类型
	Content      string    `gorm:"type:mediumtext;not null;comment:文本内容"`                                    // 文本内容
	TokenCount   int32     `gorm:"type:int;default:0;comment:Token消耗数量"`                                     // Token消耗数量
	VectorStatus int32     `gorm:"type:tinyint;default:0;comment:向量同步状态(0:未同步,1:已同步)"`                       // 向量同步状态
	IsActive     int32     `gorm:"type:tinyint;not null;default:1;index:idx_active_doc;comment:是否生效(0:失效,1:生效)"` // 是否生效
	CreatedAt    time.Time `gorm:"autoCreateTime;comment:创建时间"`                                              // 创建时间
}

func (m *KnowledgeChunkModel) TableName() string {
	return "knowledge_chunks"
}

type ChunkRepo struct {
	database.TableRepo[*KnowledgeChunkModel]
}

// BatchCreateChunks 批量保存文档切片与 Parent Chunk 上下文
func (r *ChunkRepo) BatchCreateChunks(ctx context.Context, chunks []*KnowledgeChunkModel) error {
	if len(chunks) == 0 {
		return nil
	}
	return r.GormDB(ctx).Model(&KnowledgeChunkModel{}).CreateInBatches(chunks, 100).Error
}

// GetChunksByIDs 根据 ChunkID 列表获取完整的 Chunk 记录 (回查 Parent 块上下文)
func (r *ChunkRepo) GetChunksByIDs(ctx context.Context, tenantID string, chunkIDs []string) ([]*KnowledgeChunkModel, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}
	var chunks []*KnowledgeChunkModel
	err := r.GormDB(ctx).Model(&KnowledgeChunkModel{}).
		Where("tenant_id = ? AND chunk_id IN ?", tenantID, chunkIDs).
		Find(&chunks).Error
	return chunks, err
}

// GetParentChunksByParentIDs 根据 ParentID 获取 Parent 块详细文本
func (r *ChunkRepo) GetParentChunksByParentIDs(ctx context.Context, tenantID string, parentIDs []string) (map[string]string, error) {
	if len(parentIDs) == 0 {
		return make(map[string]string), nil
	}
	var parents []*KnowledgeChunkModel
	err := r.GormDB(ctx).Model(&KnowledgeChunkModel{}).
		Where("tenant_id = ? AND chunk_id IN ?", tenantID, parentIDs).
		Find(&parents).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, p := range parents {
		result[p.ChunkID] = p.Content
	}
	return result, nil
}

// GetParentChunkModelsByParentIDs 根据 ParentID 获取 Parent 块完整模型
func (r *ChunkRepo) GetParentChunkModelsByParentIDs(ctx context.Context, tenantID string, parentIDs []string) (map[string]*KnowledgeChunkModel, error) {
	if len(parentIDs) == 0 {
		return make(map[string]*KnowledgeChunkModel), nil
	}
	var parents []*KnowledgeChunkModel
	err := r.GormDB(ctx).Model(&KnowledgeChunkModel{}).
		Where("tenant_id = ? AND chunk_id IN ?", tenantID, parentIDs).
		Find(&parents).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]*KnowledgeChunkModel)
	for _, p := range parents {
		result[p.ChunkID] = p
	}
	return result, nil
}

// DeleteChunksByDocID 根据 doc_id 删除该文档对应的全部 MySQL 块
func (r *ChunkRepo) DeleteChunksByDocID(ctx context.Context, tenantID, docID string) error {
	return r.GormDB(ctx).Model(&KnowledgeChunkModel{}).
		Where("tenant_id = ? AND doc_id = ?", tenantID, docID).
		Delete(&KnowledgeChunkModel{}).Error
}
