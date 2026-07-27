package rag

import "time"

// KnowledgeChunkModel 知识库文档切片表结构模型 (用于存储 Parent 粗粒度上下文及切片映射)
type KnowledgeChunkModel struct {
	ID           int64     `gorm:"primaryKey;autoIncrement;comment:主键ID"`                                    // 主键ID
	TenantID     string    `gorm:"type:varchar(64);not null;index:idx_tenant_chunk;comment:租户ID"`              // 租户ID
	DocID        string    `gorm:"type:varchar(64);not null;index;comment:所属文档UUID"`                           // 所属文档UUID
	ChunkID      string    `gorm:"type:varchar(64);not null;uniqueIndex;comment:切片UUID(对应向量库ID)"`              // 切片UUID
	ParentID     string    `gorm:"type:varchar(64);default:'';index;comment:父块UUID(为空表示自身为父块)"`                // 父块UUID
	ChunkIndex   int32     `gorm:"type:int;not null;comment:切片序号"`                                           // 切片序号
	Content      string    `gorm:"type:mediumtext;not null;comment:文本内容"`                                    // 文本内容
	TokenCount   int32     `gorm:"type:int;default:0;comment:Token消耗数量"`                                     // Token消耗数量
	VectorStatus int32     `gorm:"type:tinyint;default:0;comment:向量同步状态(0:未同步,1:已同步)"`                       // 向量同步状态
	CreatedAt    time.Time `gorm:"autoCreateTime;comment:创建时间"`                                              // 创建时间
}

func (m *KnowledgeChunkModel) TableName() string {
	return "knowledge_chunks"
}
