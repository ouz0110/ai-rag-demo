package rag

import (
	"context"

	"ai-rag-demo/internal/pkg/database"
)

type KnowledgeBaseRepo struct {
	database.TableRepo[*KnowledgeBaseModel]
}

type DocumentRepo struct {
	database.TableRepo[*KnowledgeDocumentModel]
}

type ChunkRepo struct {
	database.TableRepo[*KnowledgeChunkModel]
}

type RAGRepo struct {
	KBRepo    *KnowledgeBaseRepo
	DocRepo   *DocumentRepo
	ChunkRepo *ChunkRepo
	db        *database.DB
}

func NewRAGRepo(db *database.DB) *RAGRepo {
	kbRepo := &KnowledgeBaseRepo{TableRepo: database.NewTableRepo[*KnowledgeBaseModel](db)}
	docRepo := &DocumentRepo{TableRepo: database.NewTableRepo[*KnowledgeDocumentModel](db)}
	chunkRepo := &ChunkRepo{TableRepo: database.NewTableRepo[*KnowledgeChunkModel](db)}
	return &RAGRepo{
		KBRepo:    kbRepo,
		DocRepo:   docRepo,
		ChunkRepo: chunkRepo,
		db:        db,
	}
}

// CreateKnowledgeBase 创建新知识库
func (r *RAGRepo) CreateKnowledgeBase(ctx context.Context, kb *KnowledgeBaseModel) error {
	return r.KBRepo.GormDB(ctx).Model(&KnowledgeBaseModel{}).Create(kb).Error
}

// GetDefaultKnowledgeBase 获取或初始化系统默认知识库
func (r *RAGRepo) GetDefaultKnowledgeBase(ctx context.Context, tenantID string) (*KnowledgeBaseModel, error) {
	var kb KnowledgeBaseModel
	err := r.KBRepo.GormDB(ctx).Model(&KnowledgeBaseModel{}).
		Where("tenant_id = ? AND is_default = ?", tenantID, true).
		First(&kb).Error
	if err == nil {
		return &kb, nil
	}

	// 若不存在默认知识库，自动初始化一条
	defaultKB := &KnowledgeBaseModel{
		TenantID:    tenantID,
		UserID:      0,
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
func (r *RAGRepo) GetKnowledgeBaseByID(ctx context.Context, tenantID, kbID string) (*KnowledgeBaseModel, error) {
	var kb KnowledgeBaseModel
	err := r.KBRepo.GormDB(ctx).Model(&KnowledgeBaseModel{}).
		Where("tenant_id = ? AND kb_id = ?", tenantID, kbID).
		First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

// DeleteKnowledgeBase 删除自定义知识库记录 (系统默认知识库禁止删除)
func (r *RAGRepo) DeleteKnowledgeBase(ctx context.Context, tenantID, kbID string) error {
	return r.KBRepo.GormDB(ctx).Model(&KnowledgeBaseModel{}).
		Where("tenant_id = ? AND kb_id = ? AND is_default = ?", tenantID, kbID, false).
		Delete(&KnowledgeBaseModel{}).Error
}

// ListKnowledgeBases 列出用户及系统默认的知识库
func (r *RAGRepo) ListKnowledgeBases(ctx context.Context, tenantID string, userID int64) ([]*KnowledgeBaseModel, error) {
	var kbs []*KnowledgeBaseModel
	err := r.KBRepo.GormDB(ctx).Model(&KnowledgeBaseModel{}).
		Where("tenant_id = ? AND (user_id = ? OR is_default = ?)", tenantID, userID, true).
		Order("is_default desc, id desc").
		Find(&kbs).Error
	return kbs, err
}

// CreateDocument 创建文档主记录
func (r *RAGRepo) CreateDocument(ctx context.Context, doc *KnowledgeDocumentModel) error {
	return r.DocRepo.GormDB(ctx).Model(&KnowledgeDocumentModel{}).Create(doc).Error
}

// GetDocumentByDocID 查询文档主记录
func (r *RAGRepo) GetDocumentByDocID(ctx context.Context, tenantID, docID string) (*KnowledgeDocumentModel, error) {
	var doc KnowledgeDocumentModel
	err := r.DocRepo.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ? AND doc_id = ?", tenantID, docID).
		First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// UpdateDocumentStatus 更新文档处理状态
func (r *RAGRepo) UpdateDocumentStatus(ctx context.Context, tenantID, docID string, status int32, totalChunks int32, errMsg string) error {
	return r.DocRepo.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ? AND doc_id = ?", tenantID, docID).
		Updates(map[string]interface{}{
			"status":       status,
			"total_chunks": totalChunks,
			"err_msg":      errMsg,
		}).Error
}

// BatchCreateChunks 批量保存文档切片与 Parent Chunk 上下文
func (r *RAGRepo) BatchCreateChunks(ctx context.Context, chunks []*KnowledgeChunkModel) error {
	if len(chunks) == 0 {
		return nil
	}
	return r.ChunkRepo.GormDB(ctx).Model(&KnowledgeChunkModel{}).CreateInBatches(chunks, 100).Error
}

// GetChunksByIDs 根据 ChunkID 列表获取完整的 Chunk 记录 (回查 Parent 块上下文)
func (r *RAGRepo) GetChunksByIDs(ctx context.Context, tenantID string, chunkIDs []string) ([]*KnowledgeChunkModel, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}
	var chunks []*KnowledgeChunkModel
	err := r.ChunkRepo.GormDB(ctx).Model(&KnowledgeChunkModel{}).
		Where("tenant_id = ? AND chunk_id IN ?", tenantID, chunkIDs).
		Find(&chunks).Error
	return chunks, err
}

// GetParentChunksByParentIDs 根据 ParentID 获取 Parent 块详细文本
func (r *RAGRepo) GetParentChunksByParentIDs(ctx context.Context, tenantID string, parentIDs []string) (map[string]string, error) {
	if len(parentIDs) == 0 {
		return make(map[string]string), nil
	}
	var parents []*KnowledgeChunkModel
	err := r.ChunkRepo.GormDB(ctx).Model(&KnowledgeChunkModel{}).
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

// GetDocumentByFilePath 根据文件路径查询文档
func (r *RAGRepo) GetDocumentByFilePath(ctx context.Context, tenantID, filePath string) (*KnowledgeDocumentModel, error) {
	var doc KnowledgeDocumentModel
	err := r.DocRepo.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ? AND file_path = ?", tenantID, filePath).
		First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// DeleteChunksByDocID 根据 doc_id 删除该文档对应的全部 MySQL 块
func (r *RAGRepo) DeleteChunksByDocID(ctx context.Context, tenantID, docID string) error {
	return r.ChunkRepo.GormDB(ctx).Model(&KnowledgeChunkModel{}).
		Where("tenant_id = ? AND doc_id = ?", tenantID, docID).
		Delete(&KnowledgeChunkModel{}).Error
}

// DeleteDocument 根据 doc_id 删除文档主记录
func (r *RAGRepo) DeleteDocument(ctx context.Context, tenantID, docID string) error {
	return r.DocRepo.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ? AND doc_id = ?", tenantID, docID).
		Delete(&KnowledgeDocumentModel{}).Error
}

// ListAllDocuments 获取指定租户下的全部文档
func (r *RAGRepo) ListAllDocuments(ctx context.Context, tenantID string) ([]*KnowledgeDocumentModel, error) {
	var docs []*KnowledgeDocumentModel
	err := r.DocRepo.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ?", tenantID).
		Order("id desc").
		Find(&docs).Error
	return docs, err
}

// ListDocumentsByKBID 按 KBID 查询文档列表 (若 kbID 为空则获取租户下全部文档)
func (r *RAGRepo) ListDocumentsByKBID(ctx context.Context, tenantID, kbID string) ([]*KnowledgeDocumentModel, error) {
	var docs []*KnowledgeDocumentModel
	db := r.DocRepo.GormDB(ctx).Model(&KnowledgeDocumentModel{}).Where("tenant_id = ?", tenantID)
	if kbID != "" {
		db = db.Where("kb_id = ?", kbID)
	}
	err := db.Order("id desc").Find(&docs).Error
	return docs, err
}

// UpdateDocumentHash 更新文档的哈希与同步状态
func (r *RAGRepo) UpdateDocumentHash(ctx context.Context, tenantID, docID, fileHash string, status int32, totalChunks int32) error {
	return r.DocRepo.GormDB(ctx).Model(&KnowledgeDocumentModel{}).
		Where("tenant_id = ? AND doc_id = ?", tenantID, docID).
		Updates(map[string]interface{}{
			"file_hash":    fileHash,
			"status":       status,
			"total_chunks": totalChunks,
			"err_msg":      "",
		}).Error
}
