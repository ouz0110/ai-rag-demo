package nocli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"ai-rag-demo/internal/biz/nocli/vector"
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data/rag"
	"ai-rag-demo/internal/pkg/log"

	"github.com/google/uuid"
)

// KBBiz 知识库管理与自主文件上传业务 Logic
type KBBiz struct {
	cfg          *conf.Config
	ragRepo      *rag.RAGRepo
	vectorEngine *vector.VectorEngine
}

func NewKBBiz(
	cfg *conf.Config,
	ragRepo *rag.RAGRepo,
	vectorEngine *vector.VectorEngine,
) *KBBiz {
	return &KBBiz{
		cfg:          cfg,
		ragRepo:      ragRepo,
		vectorEngine: vectorEngine,
	}
}

// CreateKnowledgeBase 创建新的自定义知识库 (独立隔离开默认公共知识库)
func (b *KBBiz) CreateKnowledgeBase(ctx context.Context, name, description string) (*rag.KnowledgeBaseModel, error) {
	tenantID := "default_tenant"
	userID := int64(0)
	if ok, u := common.UserFromContext(ctx); ok && u.Openid != "" {
		tenantID = u.Openid
	}

	kbID := fmt.Sprintf("kb_%s", uuid.New().String())
	kbModel := &rag.KnowledgeBaseModel{
		TenantID:    tenantID,
		UserID:      userID,
		KBID:        kbID,
		Name:        name,
		Description: description,
		IsDefault:   false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := b.ragRepo.CreateKnowledgeBase(ctx, kbModel); err != nil {
		return nil, fmt.Errorf("create knowledge base error: %w", err)
	}

	return kbModel, nil
}

// ListKnowledgeBases 列出用户权限范围内的全部知识库 (包含系统默认知识库与用户自定义知识库)
func (b *KBBiz) ListKnowledgeBases(ctx context.Context) ([]*rag.KnowledgeBaseModel, error) {
	tenantID := "default_tenant"
	userID := int64(0)
	if ok, u := common.UserFromContext(ctx); ok && u.Openid != "" {
		tenantID = u.Openid
	}

	// 确保租户下的默认知识库已初始化
	_, _ = b.ragRepo.GetDefaultKnowledgeBase(ctx, tenantID)

	return b.ragRepo.ListKnowledgeBases(ctx, tenantID, userID)
}

// DeleteKnowledgeBase 删除自定义知识库 (系统默认公共知识库禁止删除)
func (b *KBBiz) DeleteKnowledgeBase(ctx context.Context, kbID string) error {
	tenantID := "default_tenant"
	if ok, u := common.UserFromContext(ctx); ok && u.Openid != "" {
		tenantID = u.Openid
	}

	kb, err := b.ragRepo.GetKnowledgeBaseByID(ctx, tenantID, kbID)
	if err != nil {
		return fmt.Errorf("knowledge base [%s] not found", kbID)
	}

	if kb.IsDefault {
		return fmt.Errorf("system default knowledge base cannot be deleted")
	}

	if err := b.ragRepo.DeleteKnowledgeBase(ctx, tenantID, kbID); err != nil {
		return fmt.Errorf("delete knowledge base failed: %w", err)
	}

	return nil
}

// UploadAndIngestFile 接收文件流上传保存至配置目录，并触发生产级 RAG 增量解析切片与向量化
func (b *KBBiz) UploadAndIngestFile(ctx context.Context, kbID, filename string, r io.Reader) (*rag.KnowledgeDocumentModel, error) {
	tenantID := "default_tenant"
	if ok, u := common.UserFromContext(ctx); ok && u.Openid != "" {
		tenantID = u.Openid
	}

	if kbID == "" {
		defaultKB, err := b.ragRepo.GetDefaultKnowledgeBase(ctx, tenantID)
		if err != nil {
			return nil, fmt.Errorf("fetch default kb failed: %w", err)
		}
		kbID = defaultKB.KBID
	} else {
		// 校验指定 KB 存在性
		_, err := b.ragRepo.GetKnowledgeBaseByID(ctx, tenantID, kbID)
		if err != nil {
			return nil, fmt.Errorf("knowledge base [%s] not found: %w", kbID, err)
		}
	}

	// 读取配置中的上传目录地址
	uploadDir := "./workspace/uploads"
	if b.cfg != nil && b.cfg.Source.RAG != nil && b.cfg.Source.RAG.UploadDir != "" {
		uploadDir = b.cfg.Source.RAG.UploadDir
	}

	kbTargetDir := filepath.Join(uploadDir, kbID)
	if err := os.MkdirAll(kbTargetDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload dir failed: %w", err)
	}

	safeFileName := filepath.Base(filename)
	targetFilePath := filepath.Join(kbTargetDir, safeFileName)

	// 保存文件到磁盘配置路径
	out, err := os.Create(targetFilePath)
	if err != nil {
		return nil, fmt.Errorf("create file on disk failed: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, r); err != nil {
		return nil, fmt.Errorf("save uploaded file content failed: %w", err)
	}

	log.Infof(ctx, "[Upload] File [%s] saved to disk: %s (KBID: %s)", safeFileName, targetFilePath, kbID)

	// 触发向量引擎执行生产级增量解析与切片向量化
	docModel, err := b.vectorEngine.IngestFileIncremental(ctx, tenantID, kbID, targetFilePath)
	if err != nil {
		return nil, fmt.Errorf("ingest uploaded file failed: %w", err)
	}

	return docModel, nil
}
