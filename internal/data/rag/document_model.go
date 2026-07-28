package rag

import (
	"context"
	"time"

	"ai-rag-demo/internal/pkg/database"

	"gorm.io/gorm"
)

// KnowledgeDocumentModel 知识库主文档表结构模型
type KnowledgeDocumentModel struct {
	ID              int64     `gorm:"primaryKey;autoIncrement;comment:主键ID"`                                           // 主键ID
	TenantID        string    `gorm:"type:varchar(64);not null;index:idx_tenant_doc;comment:租户ID"`                     // 租户ID
	KBID            string    `gorm:"type:varchar(64);not null;default:'kb_default_system';index:idx_kb_doc;comment:关联知识库UUID"` // 关联知识库UUID
	CollectionID    int64     `gorm:"not null;index;comment:所属知识库Collection ID"`                                      // 所属知识库Collection ID
	DocID           string    `gorm:"type:varchar(64);not null;uniqueIndex;comment:业务文档UUID"`                          // 业务文档UUID
	Title           string    `gorm:"type:varchar(255);not null;comment:文档标题"`                                         // 文档标题
	SourceType      string    `gorm:"type:varchar(32);not null;comment:文档类型(pdf,docx,md,web)"`                        // 文档类型
	DocVersion      string    `gorm:"type:varchar(32);not null;default:'v1.0';comment:文档版本号"`                          // 文档版本号
	Category        string    `gorm:"type:varchar(64);not null;default:'default';comment:文档业务分类"`                       // 文档业务分类
	Tags            string    `gorm:"type:varchar(512);default:'';comment:文档标签"`                                       // 文档标签(逗号分隔)
	IsActive        int32     `gorm:"type:tinyint;not null;default:1;comment:是否生效(0:作废/失效,1:生效)"`                      // 是否生效
	SupersedesDocID string    `gorm:"type:varchar(64);default:'';comment:被替代的旧文档UUID"`                                 // 被替代的旧文档UUID
	SourceURL       string    `gorm:"type:varchar(1024);comment:原始文件存储地址(OSS)"`                                       // 原始文件存储地址
	FilePath        string    `gorm:"type:varchar(512);index;comment:文件磁盘绝对路径"`                                       // 文件磁盘绝对路径
	FileHash        string    `gorm:"type:varchar(64);index;comment:文件内容SHA256哈希"`                                     // 文件内容SHA256哈希
	Status          int32     `gorm:"type:tinyint;default:0;comment:处理状态(0:待处理,1:解析中,2:已向量化,3:失败)"`                   // 处理状态
	TotalChunks     int32     `gorm:"type:int;default:0;comment:总切片数"`                                                 // 总切片数
	EmbeddingCost   float64   `gorm:"type:decimal(18,6);default:0.000000;comment:文档向量化花费金额"`                             // 文档向量化花费金额
	ErrMsg          string    `gorm:"type:text;comment:失败异常信息"`                                                        // 失败异常信息
	CreatedAt       time.Time `gorm:"autoCreateTime;comment:创建时间"`                                                     // 创建时间
	UpdatedAt       time.Time `gorm:"autoUpdateTime;comment:更新时间"`                                                     // 更新时间
}

func (m *KnowledgeDocumentModel) TableName() string {
	return "knowledge_documents"
}

type DocumentRepo struct {
	database.TableRepo[*KnowledgeDocumentModel]
}

// CreateDocument 创建文档主记录
func (r *DocumentRepo) CreateDocument(ctx context.Context, doc *KnowledgeDocumentModel) error {
	return r.GormDB(ctx).Model(&KnowledgeDocumentModel{}).Create(doc).Error
}

// GetDocumentByDocID 查询文档主记录
func (r *DocumentRepo) GetDocumentByDocID(ctx context.Context, tenantID, docID string) (*KnowledgeDocumentModel, error) {
	var doc KnowledgeDocumentModel
	err := r.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ? AND doc_id = ?", tenantID, docID).
		First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// GetDocumentsByDocIDs 批量查询文档记录
func (r *DocumentRepo) GetDocumentsByDocIDs(ctx context.Context, tenantID string, docIDs []string) (map[string]*KnowledgeDocumentModel, error) {
	if len(docIDs) == 0 {
		return make(map[string]*KnowledgeDocumentModel), nil
	}
	var docs []*KnowledgeDocumentModel
	err := r.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ? AND doc_id IN ?", tenantID, docIDs).
		Find(&docs).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]*KnowledgeDocumentModel)
	for _, d := range docs {
		result[d.DocID] = d
	}
	return result, nil
}

// UpdateDocumentStatus 更新文档处理状态及向量化花费
func (r *DocumentRepo) UpdateDocumentStatus(ctx context.Context, tenantID, docID string, status int32, totalChunks int32, cost float64, errMsg string) error {
	updates := map[string]interface{}{
		"status":       status,
		"total_chunks": totalChunks,
		"err_msg":      errMsg,
	}
	if cost > 0 {
		updates["embedding_cost"] = gorm.Expr("embedding_cost + ?", cost)
	}
	return r.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ? AND doc_id = ?", tenantID, docID).
		Updates(updates).Error
}

// GetDocumentByFilePath 根据文件路径查询文档
func (r *DocumentRepo) GetDocumentByFilePath(ctx context.Context, tenantID, filePath string) (*KnowledgeDocumentModel, error) {
	var doc KnowledgeDocumentModel
	err := r.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ? AND file_path = ?", tenantID, filePath).
		First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// DeleteDocument 根据 doc_id 删除文档主记录
func (r *DocumentRepo) DeleteDocument(ctx context.Context, tenantID, docID string) error {
	return r.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ? AND doc_id = ?", tenantID, docID).
		Delete(&KnowledgeDocumentModel{}).Error
}

// ListAllDocuments 获取指定租户下的全部文档
func (r *DocumentRepo) ListAllDocuments(ctx context.Context, tenantID string) ([]*KnowledgeDocumentModel, error) {
	var docs []*KnowledgeDocumentModel
	err := r.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ?", tenantID).
		Order("id desc").
		Find(&docs).Error
	return docs, err
}

// ListDocumentsByKBID 按 KBID 查询文档列表 (若 kbID 为空则获取租户下全部文档)
func (r *DocumentRepo) ListDocumentsByKBID(ctx context.Context, tenantID, kbID string) ([]*KnowledgeDocumentModel, error) {
	var docs []*KnowledgeDocumentModel
	db := r.GormDB(ctx).Model(&KnowledgeDocumentModel{}).Where("tenant_id in (?)", []string{tenantID, DefaultTenantID})
	if kbID != "" {
		db = db.Where("kb_id = ?", kbID)
	}
	err := db.Order("id desc").Find(&docs).Error
	return docs, err
}

// UpdateDocumentHash 更新文档的哈希与同步状态
func (r *DocumentRepo) UpdateDocumentHash(ctx context.Context, tenantID, docID, fileHash string, status int32, totalChunks int32) error {
	return r.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ? AND doc_id = ?", tenantID, docID).
		Updates(map[string]interface{}{
			"file_hash":    fileHash,
			"status":       status,
			"total_chunks": totalChunks,
			"err_msg":      "",
		}).Error
}
