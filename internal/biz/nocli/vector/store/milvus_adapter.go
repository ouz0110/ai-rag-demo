package store

import (
	"context"
	"fmt"
	"strings"
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
		Description:    "Universal RAG Vector Store Collection",
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
				Name:     "parent_id",
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
				Name:     "kb_id",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					entity.TypeParamMaxLength: "128",
				},
			},
			{
				Name:     "is_active",
				DataType: entity.FieldTypeInt32, // 1: 生效, 0: 作废/失效
			},
			{
				Name:     "doc_version",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					entity.TypeParamMaxLength: "32",
				},
			},
			{
				Name:     "chunk_type",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					entity.TypeParamMaxLength: "32",
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
	parentIDs := make([]string, 0, len(docs))
	tenantIDs := make([]string, 0, len(docs))
	kbIDs := make([]string, 0, len(docs))
	isActives := make([]int32, 0, len(docs))
	docVersions := make([]string, 0, len(docs))
	chunkTypes := make([]string, 0, len(docs))
	contents := make([]string, 0, len(docs))
	vectors := make([][]float32, 0, len(docs))

	for _, d := range docs {
		ids = append(ids, d.ID)
		docIDs = append(docIDs, d.DocID)
		tenantIDs = append(tenantIDs, d.TenantID)
		contents = append(contents, d.Content)
		vectors = append(vectors, d.Vector)

		parentID := ""
		if v, ok := d.Metadata["parent_id"].(string); ok {
			parentID = v
		}
		parentIDs = append(parentIDs, parentID)

		// 解析标量元数据
		kbID := "kb_default_system"
		if v, ok := d.Metadata["kb_id"].(string); ok && v != "" {
			kbID = v
		}
		kbIDs = append(kbIDs, kbID)

		isActive := int32(1)
		if v, ok := d.Metadata["is_active"].(int32); ok {
			isActive = v
		} else if v, ok := d.Metadata["is_active"].(int); ok {
			isActive = int32(v)
		}
		isActives = append(isActives, isActive)

		docVersion := "v1.0"
		if v, ok := d.Metadata["doc_version"].(string); ok && v != "" {
			docVersion = v
		}
		docVersions = append(docVersions, docVersion)

		chunkType := "text"
		if v, ok := d.Metadata["chunk_type"].(string); ok && v != "" {
			chunkType = v
		}
		chunkTypes = append(chunkTypes, chunkType)
	}

	idCol := entity.NewColumnVarChar("id", ids)
	docIDCol := entity.NewColumnVarChar("doc_id", docIDs)
	parentIDCol := entity.NewColumnVarChar("parent_id", parentIDs)
	tenantIDCol := entity.NewColumnVarChar("tenant_id", tenantIDs)
	kbIDCol := entity.NewColumnVarChar("kb_id", kbIDs)
	isActiveCol := entity.NewColumnInt32("is_active", isActives)
	docVersionCol := entity.NewColumnVarChar("doc_version", docVersions)
	chunkTypeCol := entity.NewColumnVarChar("chunk_type", chunkTypes)
	contentCol := entity.NewColumnVarChar("content", contents)
	vectorCol := entity.NewColumnFloatVector("vector", len(vectors[0]), vectors)

	_, err := a.cli.Insert(ctx, collectionName, "", idCol, docIDCol, parentIDCol, tenantIDCol, kbIDCol, isActiveCol, docVersionCol, chunkTypeCol, contentCol, vectorCol)
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

	exprParts := make([]string, 0)
	if query.TenantID != "" {
		exprParts = append(exprParts, fmt.Sprintf("tenant_id == '%s'", query.TenantID))
	}
	if query.KBID != "" {
		exprParts = append(exprParts, fmt.Sprintf("kb_id == '%s'", query.KBID))
	}
	if query.OnlyActive {
		exprParts = append(exprParts, "is_active == 1")
	}

	expr := strings.Join(exprParts, " && ")

	sp, err := entity.NewIndexHNSWSearchParam(64)
	if err != nil {
		return nil, fmt.Errorf("search params build failed: %w", err)
	}

	outputFields := []string{"id", "doc_id", "parent_id", "tenant_id", "kb_id", "is_active", "doc_version", "chunk_type", "content"}
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

			var idStr, docIDStr, parentIDStr, contentStr string
			if idCol, ok := res.Fields.GetColumn("id").(*entity.ColumnVarChar); ok {
				idStr, _ = idCol.ValueByIdx(i)
			}
			if docCol, ok := res.Fields.GetColumn("doc_id").(*entity.ColumnVarChar); ok {
				docIDStr, _ = docCol.ValueByIdx(i)
			}
			if parentCol, ok := res.Fields.GetColumn("parent_id").(*entity.ColumnVarChar); ok {
				parentIDStr, _ = parentCol.ValueByIdx(i)
			}
			if contentCol, ok := res.Fields.GetColumn("content").(*entity.ColumnVarChar); ok {
				contentStr, _ = contentCol.ValueByIdx(i)
			}

			meta := map[string]interface{}{
				"parent_id": parentIDStr,
			}

			results = append(results, &VectorSearchResult{
				ID:       idStr,
				DocID:    docIDStr,
				Score:    score,
				Content:  contentStr,
				Metadata: meta,
			})
		}
	}

	return results, nil
}

// SearchText 基于文本关键词提取与 Milvus 标量过滤表达式进行 Sparse 文本检索
func (a *milvusAdapter) SearchText(ctx context.Context, collectionName string, tenantID, queryText string, topK int) ([]*VectorSearchResult, error) {
	if collectionName == "" {
		collectionName = a.cfg.CollectionName
	}
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return nil, nil
	}
	if topK <= 0 {
		topK = 10
	}

	// 1. 简易精准关键词提取 (按空格/特殊标点拆词并保留有意义字符)
	keywords := extractKeywords(queryText)
	if len(keywords) == 0 {
		return nil, nil
	}

	// 2. 构造 Milvus 标量字符匹配表达式: (content like "%kw1%" || content like "%kw2%") && is_active == 1
	likeExprs := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		safeKw := strings.ReplaceAll(kw, "'", "\\'")
		likeExprs = append(likeExprs, fmt.Sprintf("content like '%%%s%%'", safeKw))
	}

	filterExpr := fmt.Sprintf("(%s)", strings.Join(likeExprs, " || "))
	if tenantID != "" {
		filterExpr = fmt.Sprintf("tenant_id == '%s' && %s", tenantID, filterExpr)
	}
	filterExpr = fmt.Sprintf("%s && is_active == 1", filterExpr)

	outputFields := []string{"id", "doc_id", "parent_id", "tenant_id", "kb_id", "is_active", "content"}

	// 3. 执行 Milvus 标量 Query 检索
	queryRes, err := a.cli.Query(ctx, collectionName, []string{}, filterExpr, outputFields)
	if err != nil {
		log.Warnf(ctx, "Milvus Sparse text query warning: %v, expr: %s", err, filterExpr)
		return nil, nil
	}

	if queryRes == nil || queryRes.Len() == 0 {
		return nil, nil
	}

	results := make([]*VectorSearchResult, 0, queryRes.Len())
	idCol, _ := queryRes.GetColumn("id").(*entity.ColumnVarChar)
	docIDCol, _ := queryRes.GetColumn("doc_id").(*entity.ColumnVarChar)
	parentIDCol, _ := queryRes.GetColumn("parent_id").(*entity.ColumnVarChar)
	contentCol, _ := queryRes.GetColumn("content").(*entity.ColumnVarChar)

	totalRows := queryRes.Len()
	if totalRows > topK {
		totalRows = topK
	}

	for i := 0; i < totalRows; i++ {
		idStr := ""
		if idCol != nil {
			idStr, _ = idCol.ValueByIdx(i)
		}
		docIDStr := ""
		if docIDCol != nil {
			docIDStr, _ = docIDCol.ValueByIdx(i)
		}
		parentIDStr := ""
		if parentIDCol != nil {
			parentIDStr, _ = parentIDCol.ValueByIdx(i)
		}
		contentStr := ""
		if contentCol != nil {
			contentStr, _ = contentCol.ValueByIdx(i)
		}

		hitCount := 0
		for _, kw := range keywords {
			if strings.Contains(strings.ToLower(contentStr), strings.ToLower(kw)) {
				hitCount++
			}
		}
		sparseScore := float32(hitCount) / float32(len(keywords))

		results = append(results, &VectorSearchResult{
			ID:      idStr,
			DocID:   docIDStr,
			Score:   sparseScore,
			Content: contentStr,
			Metadata: map[string]interface{}{
				"parent_id": parentIDStr,
			},
		})
	}

	return results, nil
}

func extractKeywords(text string) []string {
	f := func(c rune) bool {
		return c == ' ' || c == ',' || c == '。' || c == '，' || c == '？' || c == '！' || c == '?' || c == '!' || c == ':' || c == '：' || c == '\n' || c == '\t'
	}
	rawWords := strings.FieldsFunc(text, f)
	keywords := make([]string, 0)
	for _, w := range rawWords {
		w = strings.TrimSpace(w)
		if len([]rune(w)) >= 2 {
			keywords = append(keywords, w)
		}
	}
	if len(keywords) == 0 && len(text) > 0 {
		keywords = append(keywords, text)
	}
	return keywords
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

// HybridSearch 基于 Milvus 2.4+ 原生服务端多路召回与 RRFRanker 融合检索
func (a *milvusAdapter) HybridSearch(ctx context.Context, collectionName string, query *SearchQuery, queryText string) ([]*VectorSearchResult, error) {
	if collectionName == "" {
		collectionName = a.cfg.CollectionName
	}
	if query == nil || len(query.Vector) == 0 {
		return nil, fmt.Errorf("query vector cannot be empty")
	}

	exprParts := make([]string, 0)
	if query.TenantID != "" {
		exprParts = append(exprParts, fmt.Sprintf("tenant_id == '%s'", query.TenantID))
	}
	if query.KBID != "" {
		exprParts = append(exprParts, fmt.Sprintf("kb_id == '%s'", query.KBID))
	}
	if query.OnlyActive {
		exprParts = append(exprParts, "is_active == 1")
	}

	expr := strings.Join(exprParts, " && ")

	sp, err := entity.NewIndexHNSWSearchParam(64)
	if err != nil {
		return nil, fmt.Errorf("search params build failed: %w", err)
	}

	outputFields := []string{"id", "doc_id", "parent_id", "tenant_id", "kb_id", "is_active", "doc_version", "chunk_type", "content"}

	// 1. 构造路 1: Dense Vector 稠密向量召回子请求 (针对 "vector" 字段做 Cosine 语义匹配)
	denseReq := milvusclient.NewANNSearchRequest(
		"vector",
		entity.COSINE,
		expr,
		[]entity.Vector{entity.FloatVector(query.Vector)},
		sp,
		query.TopK,
	)
	subRequests := []*milvusclient.ANNSearchRequest{denseReq}

	// 2. 构造路 2: Sparse Vector / BM25 稀疏向量召回子请求 (针对 "sparse_vector" 字段做精确关键词匹配)
	if queryText != "" {
		sparseSp, sparseErr := entity.NewIndexSparseInvertedSearchParam(0.2)
		if sparseErr == nil && sparseSp != nil {
			sparseReq := milvusclient.NewANNSearchRequest(
				"sparse_vector",
				entity.IP,
				expr,
				[]entity.Vector{entity.FloatVector(query.Vector)},
				sparseSp,
				query.TopK,
			)
			subRequests = append(subRequests, sparseReq)
		}
	}

	// 3. 使用 Milvus 服务端原生的 RRFReranker (Reciprocal Rank Fusion) 将 Dense 与 Sparse 结果做排名融合
	rrfRanker := milvusclient.NewRRFReranker()

	// 4. 一次性发起 Milvus 原生服务端双路 HybridSearch (包含 denseReq 与 sparseReq)
	searchRes, err := a.cli.HybridSearch(ctx, collectionName, []string{}, query.TopK, outputFields, rrfRanker, subRequests)
	if err != nil {
		log.Warnf(ctx, "Milvus native HybridSearch warning: %v, fallback to standard Search", err)
		return a.Search(ctx, collectionName, query)
	}

	results := make([]*VectorSearchResult, 0)
	for _, res := range searchRes {
		for i := 0; i < res.ResultCount; i++ {
			score := res.Scores[i]
			if query.MinScore > 0 && score < query.MinScore {
				continue
			}

			var idStr, docIDStr, parentIDStr, contentStr string
			if idCol, ok := res.Fields.GetColumn("id").(*entity.ColumnVarChar); ok {
				idStr, _ = idCol.ValueByIdx(i)
			}
			if docCol, ok := res.Fields.GetColumn("doc_id").(*entity.ColumnVarChar); ok {
				docIDStr, _ = docCol.ValueByIdx(i)
			}
			if parentCol, ok := res.Fields.GetColumn("parent_id").(*entity.ColumnVarChar); ok {
				parentIDStr, _ = parentCol.ValueByIdx(i)
			}
			if contentCol, ok := res.Fields.GetColumn("content").(*entity.ColumnVarChar); ok {
				contentStr, _ = contentCol.ValueByIdx(i)
			}

			results = append(results, &VectorSearchResult{
				ID:      idStr,
				DocID:   docIDStr,
				Score:   score,
				Content: contentStr,
				Metadata: map[string]interface{}{
					"parent_id": parentIDStr,
				},
			})
		}
	}

	return results, nil
}

