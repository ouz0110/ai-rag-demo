package rag

import (
	"context"
	"time"

	"ai-rag-demo/internal/pkg/database"
)

// KnowledgeBaseModel 知识库主表结构体 (独立隔离系统默认知识库与用户新增的自定义知识库)
type KnowledgeBaseModel struct {
	ID          int64     `gorm:"primaryKey;autoIncrement;comment:主键ID"`
	TenantID    string    `gorm:"type:varchar(64);not null;index:idx_tenant_kb;comment:租户ID"`
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

type KnowledgeBaseRepo struct {
	database.TableRepo[*KnowledgeBaseModel]
}

// CreateKnowledgeBase 创建新知识库
func (r *KnowledgeBaseRepo) CreateKnowledgeBase(ctx context.Context, kb *KnowledgeBaseModel) error {
	return r.GormDB(ctx).Model(&KnowledgeBaseModel{}).Create(kb).Error
}

// GetDefaultKnowledgeBase 获取或初始化系统默认知识库
func (r *KnowledgeBaseRepo) GetDefaultKnowledgeBase(ctx context.Context, tenantID string) (*KnowledgeBaseModel, error) {
	var kb KnowledgeBaseModel
	err := r.GormDB(ctx).Model(&KnowledgeBaseModel{}).
		Where("tenant_id = ? AND is_default = ?", tenantID, true).
		First(&kb).Error
	if err == nil {
		return &kb, nil
	}

	// 若不存在默认知识库，自动初始化一条
	defaultKB := &KnowledgeBaseModel{
		TenantID:    tenantID,
		KBID:        DefaultKBID,
		Name:        "系统默认知识库",
		Description: "系统默认公共知识库",
		IsDefault:   true,
	}
	if createErr := r.CreateKnowledgeBase(ctx, defaultKB); createErr != nil {
		return nil, createErr
	}
	return defaultKB, nil
}

// GetKnowledgeBaseByID 根据 KBID 获取知识库详情
func (r *KnowledgeBaseRepo) GetKnowledgeBaseByID(ctx context.Context, tenantID, kbID string) (*KnowledgeBaseModel, error) {
	var kb KnowledgeBaseModel
	err := r.GormDB(ctx).Model(&KnowledgeBaseModel{}).
		Where("tenant_id = ? AND kb_id = ?", tenantID, kbID).
		First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

// DeleteKnowledgeBase 删除自定义知识库记录 (系统默认知识库禁止删除)
func (r *KnowledgeBaseRepo) DeleteKnowledgeBase(ctx context.Context, tenantID, kbID string) error {
	return r.GormDB(ctx).Model(&KnowledgeBaseModel{}).
		Where("tenant_id = ? AND kb_id = ? AND is_default = ?", tenantID, kbID, false).
		Delete(&KnowledgeBaseModel{}).Error
}

// ListKnowledgeBases 列出租户下的所有知识库
func (r *KnowledgeBaseRepo) ListKnowledgeBases(ctx context.Context, tenantID string) ([]*KnowledgeBaseModel, error) {
	var kbs []*KnowledgeBaseModel
	err := r.GormDB(ctx).Model(&KnowledgeBaseModel{}).
		Where("tenant_id = ?", tenantID).
		Order("is_default desc, id desc").
		Find(&kbs).Error
	return kbs, err
}
