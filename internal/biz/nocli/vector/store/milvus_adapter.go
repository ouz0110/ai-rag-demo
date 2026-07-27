package store

import (
	"context"
	"fmt"
	"sync"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/log"

	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type milvusAdapter struct {
	cli milvusclient.Client
	cfg *conf.MilvusConfig
	mu  sync.RWMutex
}

func newMilvusAdapter(cfg *conf.MilvusConfig) (VectorStore, error) {
	if cfg == nil || cfg.Address == "" {
		return nil, fmt.Errorf("milvus configuration is empty or address is missing")
	}

	ctx := context.Background()
	cli, err := milvusclient.NewClient(ctx, milvusclient.Config{
		Address:  cfg.Address,
		Username: cfg.Username,
		Password: cfg.Password,
		DBName:   cfg.DBName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect milvus server: %w", err)
	}

	log.Infof(ctx, "Milvus client connected successfully to %s", cfg.Address)
	return &milvusAdapter{
		cli: cli,
		cfg: cfg,
	}, nil
}

func (a *milvusAdapter) HasCollection(ctx context.Context, collectionName string) (bool, error) {
	if collectionName == "" {
		collectionName = a.cfg.CollectionName
	}
	return a.cli.HasCollection(ctx, collectionName)
}

func (a *milvusAdapter) CreateCollection(ctx context.Context, collectionName string, dim int) error {
	if collectionName == "" {
		collectionName = a.cfg.CollectionName
	}
	if dim <= 0 {
		dim = a.cfg.Dimension
	}

	exists, err := a.cli.HasCollection(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("check collection error: %w", err)
	}
	if exists {
		return nil
	}

	// Schema 定义
	schema := &entity.Schema{
		CollectionName: collectionName,
		Description:    "RAG Vector Store Collection",
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeVarChar,
				PrimaryKey: true,
				AutoID:     false,
				TypeParams: map[string]string{
					entity.TypeParamMaxLength: "128",
				},
			},
			{
				Name:     "doc_id",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					entity.TypeParamMaxLength: "128",
				},
			},
			{
				Name:           "tenant_id",
				DataType:       entity.FieldTypeVarChar,
				IsPartitionKey: true, // 开启 Milvus Partition Key 级别多租户：触发底层分区裁剪 (Partition Pruning)，提升海量租户检索效率
				TypeParams: map[string]string{
					entity.TypeParamMaxLength: "128",
				},
			},
			{
				Name:     "content",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					entity.TypeParamMaxLength: "8192",
				},
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					entity.TypeParamDim: fmt.Sprintf("%d", dim),
				},
			},
		},
	}

	if err := a.cli.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
		return fmt.Errorf("create milvus collection failed: %w", err)
	}

	// 创建 HNSW 向量索引
	idx, err := entity.NewIndexHNSW(entity.COSINE, 16, 200)
	if err != nil {
		return fmt.Errorf("create hnsw index error: %w", err)
	}

	if err := a.cli.CreateIndex(ctx, collectionName, "vector", idx, false); err != nil {
		return fmt.Errorf("milvus create index error: %w", err)
	}

	// 加载 Collection 至内存
	if err := a.cli.LoadCollection(ctx, collectionName, false); err != nil {
		return fmt.Errorf("load collection error: %w", err)
	}

	log.Infof(ctx, "Milvus collection [%s] created and loaded successfully", collectionName)
	return nil
}

func (a *milvusAdapter) Upsert(ctx context.Context, collectionName string, docs []*VectorDocument) error {
	if len(docs) == 0 {
		return nil
	}
	if collectionName == "" {
		collectionName = a.cfg.CollectionName
	}

	ids := make([]string, 0, len(docs))
	docIDs := make([]string, 0, len(docs))
	tenantIDs := make([]string, 0, len(docs))
	contents := make([]string, 0, len(docs))
	vectors := make([][]float32, 0, len(docs))

	for _, d := range docs {
		ids = append(ids, d.ID)
		docIDs = append(docIDs, d.DocID)
		tenantIDs = append(tenantIDs, d.TenantID)
		contents = append(contents, d.Content)
		vectors = append(vectors, d.Vector)
	}

	idCol := entity.NewColumnVarChar("id", ids)
	docIDCol := entity.NewColumnVarChar("doc_id", docIDs)
	tenantIDCol := entity.NewColumnVarChar("tenant_id", tenantIDs)
	contentCol := entity.NewColumnVarChar("content", contents)
	vectorCol := entity.NewColumnFloatVector("vector", len(vectors[0]), vectors)

	_, err := a.cli.Insert(ctx, collectionName, "", idCol, docIDCol, tenantIDCol, contentCol, vectorCol)
	if err != nil {
		return fmt.Errorf("milvus insert error: %w", err)
	}

	// 保证可见性
	_ = a.cli.Flush(ctx, collectionName, false)
	return nil
}

func (a *milvusAdapter) BatchDelete(ctx context.Context, collectionName string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if collectionName == "" {
		collectionName = a.cfg.CollectionName
	}

	expr := fmt.Sprintf("id in %s", formatStringSlice(ids))
	return a.cli.Delete(ctx, collectionName, "", expr)
}

func (a *milvusAdapter) DeleteByDocID(ctx context.Context, collectionName string, tenantID, docID string) error {
	if collectionName == "" {
		collectionName = a.cfg.CollectionName
	}

	expr := fmt.Sprintf("tenant_id == '%s' && doc_id == '%s'", tenantID, docID)
	return a.cli.Delete(ctx, collectionName, "", expr)
}

func (a *milvusAdapter) Search(ctx context.Context, collectionName string, query *SearchQuery) ([]*VectorSearchResult, error) {
	if collectionName == "" {
		collectionName = a.cfg.CollectionName
	}
	if query == nil || len(query.Vector) == 0 {
		return nil, fmt.Errorf("query vector cannot be empty")
	}

	expr := ""
	if query.TenantID != "" {
		expr = fmt.Sprintf("tenant_id == '%s'", query.TenantID)
	}

	sp, err := entity.NewIndexHNSWSearchParam(64)
	if err != nil {
		return nil, fmt.Errorf("search params build failed: %w", err)
	}

	outputFields := []string{"id", "doc_id", "tenant_id", "content"}
	vectors := []entity.Vector{entity.FloatVector(query.Vector)}

	searchRes, err := a.cli.Search(ctx, collectionName, []string{}, expr, outputFields, vectors, "vector", entity.COSINE, query.TopK, sp)
	if err != nil {
		return nil, fmt.Errorf("milvus search failed: %w", err)
	}

	results := make([]*VectorSearchResult, 0)
	for _, res := range searchRes {
		for i := 0; i < res.ResultCount; i++ {
			score := res.Scores[i]
			if query.MinScore > 0 && score < query.MinScore {
				continue
			}

			var idStr, docIDStr, contentStr string
			if idCol, ok := res.Fields.GetColumn("id").(*entity.ColumnVarChar); ok {
				idStr, _ = idCol.ValueByIdx(i)
			}
			if docCol, ok := res.Fields.GetColumn("doc_id").(*entity.ColumnVarChar); ok {
				docIDStr, _ = docCol.ValueByIdx(i)
			}
			if contentCol, ok := res.Fields.GetColumn("content").(*entity.ColumnVarChar); ok {
				contentStr, _ = contentCol.ValueByIdx(i)
			}

			results = append(results, &VectorSearchResult{
				ID:      idStr,
				DocID:   docIDStr,
				Score:   score,
				Content: contentStr,
			})
		}
	}

	return results, nil
}

func formatStringSlice(slice []string) string {
	res := "["
	for i, s := range slice {
		if i > 0 {
			res += ", "
		}
		res += fmt.Sprintf("'%s'", s)
	}
	res += "]"
	return res
}
