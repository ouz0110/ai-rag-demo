package rag

import "time"

// KnowledgeBaseModel 知识库主表结构体 (独立隔离系统默认知识库与用户新增的自定义知识库)
type KnowledgeBaseModel struct {
	ID          int64     `gorm:"primaryKey;autoIncrement;comment:主键ID"`
	TenantID    string    `gorm:"type:varchar(64);not null;index:idx_tenant_kb;comment:租户ID"`
	UserID      int64     `gorm:"not null;index;comment:创建用户ID"`
	KBID        string    `gorm:"type:varchar(64);not null;uniqueIndex;comment:知识库UUID标识"`
	Name        string    `gorm:"type:varchar(128);not null;comment:知识库名称"`
	Description string    `gorm:"type:varchar(512);comment:知识库描述"`
	IsDefault   bool      `gorm:"type:boolean;default:false;index;comment:是否为系统默认公共知识库"`
	CreatedAt   time.Time `gorm:"autoCreateTime;comment:创建时间"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime;comment:更新时间"`
}

func (m *KnowledgeBaseModel) TableName() string {
	return "knowledge_bases"
}
