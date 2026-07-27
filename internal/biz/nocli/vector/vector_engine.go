package vector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-rag-demo/internal/biz/nocli/vector/chunker"
	"ai-rag-demo/internal/biz/nocli/vector/embedder"
	"ai-rag-demo/internal/biz/nocli/vector/parser"
	"ai-rag-demo/internal/biz/nocli/vector/rerank"
	"ai-rag-demo/internal/biz/nocli/vector/retriever"
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
	ragData "ai-rag-demo/internal/data/rag"
	vectorData "ai-rag-demo/internal/biz/nocli/vector/store"
	"ai-rag-demo/internal/pkg/log"

	"github.com/google/uuid"
)

// RAGContext 最终输出给 LLM 组装 Prompt 的上下文数据结构
type RAGContext struct {
	ChunkID      string                 `json:"chunk_id"`      // 命中切片唯一 ID
	DocID        string                 `json:"doc_id"`        // 所属主文档 ID
	ParentID     string                 `json:"parent_id"`     // 父切片 ID
	ChildContent string                 `json:"child_content"` // 细粒度子切片文本 (向量匹配源)
	FullContext  string                 `json:"full_context"`  // 回查 Parent 数据库拿到的粗粒度完整上下文
	Score        float32                `json:"score"`         // 相似度或 RRF 综合得分
	Metadata     map[string]interface{} `json:"metadata"`      // 标量元数据键值对 (包含溯源来源文件等)
}

// VectorEngine 生产级向量与 RAG 业务门面编排引擎
type VectorEngine struct {
	cfg             *conf.Config                // 系统配置句柄
	vectorStore     vectorData.VectorStore      // 向量存储驱动接口 (Milvus / Qdrant)
	ragRepo         *ragData.RAGRepo            // MySQL 文档与 Chunk 持久化仓库
	embedder        *embedder.Embedder          // 向量 Embedding 调度计算器
	staticChunker   *chunker.ParentChildChunker // 静态父子切片算子
	semanticChunker *chunker.SemanticChunker    // 两阶段语义感知动态切片算子
	retriever       *retriever.HybridRetriever  // 混合检索与 RRF 融合执行器
	reranker        rerank.Reranker             // 独立 Rerank 重排策略 (支持 LLM / BGE)
	parserFactory   *parser.ParserFactory       // 异构文件格式解析策略工厂
}

func NewVectorEngine(
	cfg *conf.Config,
	vectorStore vectorData.VectorStore,
	ragRepo *ragData.RAGRepo,
) *VectorEngine {
	emb := embedder.NewEmbedder(cfg)
	chk := chunker.NewStaticChunkerFromConfig(cfg)
	semChk := chunker.NewSemanticChunkerFromConfig(cfg, emb)
	ret := retriever.NewHybridRetriever(vectorStore)
	rrk := rerank.NewReranker(cfg)
	pFactory := parser.NewParserFactory()

	engine := &VectorEngine{
		cfg:             cfg,
		vectorStore:     vectorStore,
		ragRepo:         ragRepo,
		embedder:        emb,
		staticChunker:   chk,
		semanticChunker: semChk,
		retriever:       ret,
		reranker:        rrk,
		parserFactory:   pFactory,
	}

	// 启动时判断配置，若开启 auto_reload，使用 safe goroutine 异步扫描重载知识库目录
	if cfg != nil && cfg.Source.RAG != nil && cfg.Source.RAG.AutoReload {
		common.RunInGoroutine(context.Background(), func(ctx context.Context) {
			engine.AutoReloadKnowledgeBase(ctx)
		})
	}

	return engine
}

// CalculateContentHash 计算文件内容的 SHA256 哈希值
func CalculateContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// IngestFileIncremental 生产级增量同步方法：基于 SHA256 哈希对比，文件未变动不删除/重生成 chunk；变动后仅删除对应文件 chunk 并重建
func (e *VectorEngine) IngestFileIncremental(ctx context.Context, tenantID, kbID, filePath string) (*ragData.KnowledgeDocumentModel, error) {
	if tenantID == "" {
		tenantID = DefaultTenantID
	}
	if kbID == "" {
		kbID = DefaultKBID
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	fileBytes, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file failed [%s]: %w", absPath, err)
	}

	currentHash := CalculateContentHash(fileBytes)
	fileTitle := filepath.Base(absPath)
	ext := strings.TrimPrefix(filepath.Ext(absPath), ".")
	if ext == "" {
		ext = "txt"
	}

	collectionName := e.getCollectionName()

	// 1. 检索数据库是否已存在该文件的记录
	existingDoc, err := e.ragRepo.GetDocumentByFilePath(ctx, tenantID, absPath)
	if err == nil && existingDoc != nil {
		// 【防重复处理关键】：内容哈希一致，无变动 -> 跳过全量重新切片与向量化！
		if existingDoc.FileHash == currentHash && existingDoc.Status == 2 {
			log.Infof(ctx, "[Incremental Sync] File [%s] is unchanged (SHA256: %s), skipping re-indexing.", fileTitle, currentHash[:8])
			return existingDoc, nil
		}

		// 【变动处理关键】：内容哈希不一致 -> 属于最小粒度变动，仅物理清理旧文件对应的 chunk 和向量
		log.Infof(ctx, "[Incremental Sync] File [%s] changed (old: %s, new: %s). Purging old chunks & re-indexing.", fileTitle, existingDoc.FileHash[:8], currentHash[:8])

		// 清理旧向量数据 (Milvus)
		if e.vectorStore != nil {
			if delErr := e.vectorStore.DeleteByDocID(ctx, collectionName, tenantID, existingDoc.DocID); delErr != nil {
				log.Warnf(ctx, "Delete old vectors for doc [%s] warn: %v", existingDoc.DocID, delErr)
			}
		}
		// 清理 MySQL 旧块记录
		if delErr := e.ragRepo.DeleteChunksByDocID(ctx, tenantID, existingDoc.DocID); delErr != nil {
			log.Warnf(ctx, "Delete old mysql chunks for doc [%s] warn: %v", existingDoc.DocID, delErr)
		}

		// 执行增量更新写入
		return e.processAndSaveDocument(ctx, existingDoc, string(fileBytes), currentHash, absPath, collectionName)
	}

	// 2. 属于全新文件 -> 创建新 Doc 记录并处理
	newDocID := fmt.Sprintf("doc_%s", uuid.New().String())
	newDoc := &ragData.KnowledgeDocumentModel{
		TenantID:   tenantID,
		KBID:       kbID,
		DocID:      newDocID,
		Title:      fileTitle,
		SourceType: ext,
		DocVersion: "v1.0",
		Category:   "default",
		IsActive:   1,
		FilePath:   absPath,
		FileHash:   currentHash,
		Status:     1, // 解析中
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := e.ragRepo.CreateDocument(ctx, newDoc); err != nil {
		return nil, fmt.Errorf("create new document record failed: %w", err)
	}

	return e.processAndSaveDocument(ctx, newDoc, string(fileBytes), currentHash, absPath, collectionName)
}

func (e *VectorEngine) processAndSaveDocument(ctx context.Context, doc *ragData.KnowledgeDocumentModel, rawContent, contentHash, absPath, collectionName string) (*ragData.KnowledgeDocumentModel, error) {
	// 1. 调用策略工厂匹配对应格式的 DocumentParser 进行差异化解析 (如 CSV 扁平化、JSON 结构树转换、TXT 纯文本)
	parsedDoc, err := e.parserFactory.GetParser(doc.Title).Parse(ctx, strings.NewReader(rawContent), doc.Title)
	flattenedText := rawContent
	if err == nil && parsedDoc != nil && parsedDoc.Content != "" {
		flattenedText = parsedDoc.Content
	}

	// 2. 【生产级关键项】：优先执行两阶段语义感知动态边界切片 (底包 512 字符 + 余弦相似度高分合并/话题断层割断)
	var splitRes *chunker.ParentChildResult
	if e.semanticChunker != nil {
		semRes, semErr := e.semanticChunker.SplitWithSemantics(ctx, flattenedText)
		if semErr == nil && semRes != nil && len(semRes.ChildChunks) > 0 {
			splitRes = semRes
		} else if semErr != nil {
			log.Warnf(ctx, "Semantic chunker failed: %v, falling back to standard ParentChildChunker", semErr)
		}
	}
	if splitRes == nil {
		splitRes = e.staticChunker.Split(flattenedText)
	}
	totalChunks := len(splitRes.ChildChunks)

	// 3. 【生产级关键项】：前缀元数据硬编码注入，消除大模型缺乏上下文瞎编概率
	for _, p := range splitRes.ParentChunks {
		p.Content = chunker.InjectMetadataPrefix(doc.Title, p.Content)
	}
	for _, c := range splitRes.ChildChunks {
		c.Content = chunker.InjectMetadataPrefix(doc.Title, c.Content)
	}

	// 构造 MySQL 块 (附带切片 SHA256 Hash 与类型)
	chunkModels := make([]*ragData.KnowledgeChunkModel, 0, len(splitRes.ParentChunks)+len(splitRes.ChildChunks))
	for _, p := range splitRes.ParentChunks {
		pHash := CalculateContentHash([]byte(p.Content))
		chunkModels = append(chunkModels, &ragData.KnowledgeChunkModel{
			TenantID:     doc.TenantID,
			DocID:        doc.DocID,
			ChunkID:      p.ChunkID,
			ParentID:     "",
			ChunkIndex:   p.ChunkIndex,
			ChunkHash:    pHash,
			ChunkType:    "text",
			Content:      p.Content,
			TokenCount:   p.TokenCount,
			VectorStatus: 0,
			IsActive:     1,
		})
	}
	for _, c := range splitRes.ChildChunks {
		cHash := CalculateContentHash([]byte(c.Content))
		chunkModels = append(chunkModels, &ragData.KnowledgeChunkModel{
			TenantID:     doc.TenantID,
			DocID:        doc.DocID,
			ChunkID:      c.ChunkID,
			ParentID:     c.ParentID,
			ChunkIndex:   c.ChunkIndex,
			ChunkHash:    cHash,
			ChunkType:    "text",
			Content:      c.Content,
			TokenCount:   c.TokenCount,
			VectorStatus: 1,
			IsActive:     1,
		})
	}

	if err := e.ragRepo.BatchCreateChunks(ctx, chunkModels); err != nil {
		_ = e.ragRepo.UpdateDocumentStatus(ctx, doc.TenantID, doc.DocID, 3, 0, err.Error())
		return nil, fmt.Errorf("batch save chunks error: %w", err)
	}

	// 向量化 Child Chunks
	childTexts := make([]string, len(splitRes.ChildChunks))
	for i, c := range splitRes.ChildChunks {
		childTexts[i] = c.Content
	}

	vectors, err := e.embedder.BatchGenerateEmbeddings(ctx, childTexts)
	if err != nil {
		_ = e.ragRepo.UpdateDocumentStatus(ctx, doc.TenantID, doc.DocID, 3, 0, err.Error())
		return nil, fmt.Errorf("generate embeddings error: %w", err)
	}

	// 构建 VectorDocument (注入完整的标量 Metadata 属性)
	vecDocs := make([]*vectorData.VectorDocument, len(splitRes.ChildChunks))
	for i, c := range splitRes.ChildChunks {
		vecDocs[i] = &vectorData.VectorDocument{
			ID:       c.ChunkID,
			DocID:    doc.DocID,
			TenantID: doc.TenantID,
			Vector:   vectors[i],
			Content:  c.Content,
			Metadata: map[string]interface{}{
				"parent_id":   c.ParentID,
				"doc_id":      doc.DocID,
				"tenant_id":   doc.TenantID,
				"kb_id":       doc.KBID,
				"is_active":   int32(1),
				"doc_version": doc.DocVersion,
				"chunk_type":  "text",
			},
		}
	}

	if e.vectorStore != nil {
		dim := e.embedder.Dimension()
		if len(vectors) > 0 && len(vectors[0]) > 0 {
			dim = len(vectors[0])
		}
		_ = e.vectorStore.CreateCollection(ctx, collectionName, dim)
		if err := e.vectorStore.Upsert(ctx, collectionName, vecDocs); err != nil {
			_ = e.ragRepo.UpdateDocumentStatus(ctx, doc.TenantID, doc.DocID, 3, 0, err.Error())
			return nil, fmt.Errorf("upsert vector store error: %w", err)
		}
	}

	// 更新 Hash 与状态
	_ = e.ragRepo.UpdateDocumentHash(ctx, doc.TenantID, doc.DocID, contentHash, 2, int32(totalChunks))
	doc.FileHash = contentHash
	doc.Status = 2
	doc.TotalChunks = int32(totalChunks)

	log.Infof(ctx, "Document [%s] (%s) ingested successfully with %d child chunks.", doc.Title, doc.DocID, totalChunks)
	return doc, nil
}

// AutoReloadKnowledgeBase 增量扫描并自动重载配置目录下的知识库文件
func (e *VectorEngine) AutoReloadKnowledgeBase(ctx context.Context) {
	knowledgeDir := "./workspace/knowledge"
	if e.cfg != nil && e.cfg.Source.RAG != nil && e.cfg.Source.RAG.KnowledgeDir != "" {
		knowledgeDir = e.cfg.Source.RAG.KnowledgeDir
	}

	absDir, err := filepath.Abs(knowledgeDir)
	if err != nil {
		absDir = knowledgeDir
	}

	log.Infof(ctx, "[AutoReload] Starting knowledge base incremental scan in directory: %s", absDir)

	if _, err := os.Stat(absDir); os.IsNotExist(err) {
		_ = os.MkdirAll(absDir, 0755)
		log.Infof(ctx, "[AutoReload] Knowledge directory created: %s", absDir)
		return
	}

	tenantID := DefaultTenantID
	foundFiles := make(map[string]bool)

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".md" || ext == ".txt" || ext == ".json" || ext == ".pdf" || ext == ".docx" || ext == ".csv" || ext == ".tsv" {
			foundFiles[path] = true
			_, ingestErr := e.IngestFileIncremental(ctx, tenantID, DefaultKBID, path)
			if ingestErr != nil {
				log.Errorf(ctx, "[AutoReload] Ingest file [%s] error: %v", path, ingestErr)
			}
		}
		return nil
	})

	if err != nil {
		log.Errorf(ctx, "[AutoReload] Walk directory error: %v", err)
	}

	// 【清理失效文件】：对比数据库与磁盘，若文件已被删除，清理对应的物理数据库与向量 chunk
	existingDocs, err := e.ragRepo.ListAllDocuments(ctx, tenantID)
	if err == nil {
		collectionName := e.getCollectionName()
		for _, d := range existingDocs {
			if d.FilePath != "" && !foundFiles[d.FilePath] {
				log.Infof(ctx, "[AutoReload] Purging deleted file record [%s] (DocID: %s)", d.FilePath, d.DocID)
				if e.vectorStore != nil {
					_ = e.vectorStore.DeleteByDocID(ctx, collectionName, tenantID, d.DocID)
				}
				_ = e.ragRepo.DeleteChunksByDocID(ctx, tenantID, d.DocID)
				_ = e.ragRepo.DeleteDocument(ctx, tenantID, d.DocID)
			}
		}
	}

	log.Infof(ctx, "[AutoReload] Knowledge base incremental sync completed.")
}

func (e *VectorEngine) getCollectionName() string {
	collectionName := DefaultCollectionName
	if e.cfg != nil && e.cfg.Source.VectorDB != nil && e.cfg.Source.VectorDB.Milvus != nil {
		if e.cfg.Source.VectorDB.Milvus.CollectionName != "" {
			collectionName = e.cfg.Source.VectorDB.Milvus.CollectionName
		}
	}
	return collectionName
}

// DeleteDocVectors 从向量索引中清理掉指定 docID 的全部向量切片
func (e *VectorEngine) DeleteDocVectors(ctx context.Context, tenantID, docID string) error {
	if e.vectorStore != nil {
		collectionName := e.getCollectionName()
		return e.vectorStore.DeleteByDocID(ctx, collectionName, tenantID, docID)
	}
	return nil
}

// IngestDocument 原基础文档接入接口
func (e *VectorEngine) IngestDocument(ctx context.Context, tenantID, docID, title, sourceType, sourceURL, rawContent string) (*ragData.KnowledgeDocumentModel, error) {
	docHash := CalculateContentHash([]byte(rawContent))
	docModel := &ragData.KnowledgeDocumentModel{
		TenantID:   tenantID,
		DocID:      docID,
		Title:      title,
		SourceType: sourceType,
		SourceURL:  sourceURL,
		FileHash:   docHash,
		Status:     1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := e.ragRepo.CreateDocument(ctx, docModel); err != nil {
		return nil, fmt.Errorf("save document record error: %w", err)
	}

	return e.processAndSaveDocument(ctx, docModel, rawContent, docHash, "", e.getCollectionName())
}

// RetrieveContext 执行生产级混合检索、1.5s SRE 硬超时降级与首尾强化 Context 组装 (解决 Lost in the Middle)
func (e *VectorEngine) RetrieveContext(ctx context.Context, tenantID, queryText string, topK int) ([]*RAGContext, error) {
	if tenantID == "" || queryText == "" {
		return nil, fmt.Errorf("empty query params")
	}

	scoreThreshold := float32(0.1)
	if e.cfg != nil && e.cfg.Source.RAG != nil {
		if topK <= 0 && e.cfg.Source.RAG.TopK > 0 {
			topK = e.cfg.Source.RAG.TopK
		}
		if e.cfg.Source.RAG.ScoreThreshold > 0 {
			scoreThreshold = e.cfg.Source.RAG.ScoreThreshold
		}
	}
	if topK <= 0 {
		topK = 5
	}

	collectionName := e.getCollectionName()

	// 【SRE 硬超时防护】：设置 1.5 秒硬超时，超时触发降级返回空或兜底结果，防止拖垮在线 QPS
	retCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	// 1. 生成 Query 的 Embedding 向量
	queryVec, err := e.embedder.GenerateEmbedding(retCtx, queryText)
	if err != nil {
		log.Warnf(ctx, "Generate query embedding failed or timed out: %v", err)
		return nil, fmt.Errorf("generate query embedding failed: %w", err)
	}

	// 2. 构建向量检索请求 (自动带上 OnlyActive: true 过滤掉被作废或失效的切片)
	searchQuery := &vectorData.SearchQuery{
		TenantID:   tenantID,
		Vector:     queryVec,
		TopK:       topK * 2,
		MinScore:   scoreThreshold,
		OnlyActive: true,
	}

	// 3. 执行 Hybrid Retriever 召回
	scoredChunks, err := e.retriever.Retrieve(retCtx, collectionName, searchQuery, queryText)
	if err != nil {
		log.Warnf(ctx, "Hybrid retrieval failed or timed out: %v", err)
		return nil, fmt.Errorf("hybrid retrieval failed: %w", err)
	}

	if len(scoredChunks) == 0 {
		return nil, nil
	}

	// 4. 【二阶段重排 (Rerank)】：若启用独立 Reranker (如 LLM 打分策略)，对粗筛候选集深度二次打分
	if e.reranker != nil && len(scoredChunks) > 1 {
		candidates := make([]*rerank.RerankCandidate, len(scoredChunks))
		for i, sc := range scoredChunks {
			candidates[i] = &rerank.RerankCandidate{
				ID:       sc.ChunkID,
				DocID:    sc.DocID,
				ParentID: sc.ParentID,
				Content:  sc.Content,
				Score:    sc.Score,
				Metadata: sc.Metadata,
			}
		}

		rerankTimeout := 1000 * time.Millisecond
		if e.cfg != nil && e.cfg.Source.RAG != nil && e.cfg.Source.RAG.Rerank != nil && e.cfg.Source.RAG.Rerank.Timeout > 0 {
			rerankTimeout = time.Duration(e.cfg.Source.RAG.Rerank.Timeout) * time.Millisecond
		}

		rrCtx, rrCancel := context.WithTimeout(retCtx, rerankTimeout)
		rerankedCandidates, rrErr := e.reranker.Rerank(rrCtx, queryText, candidates)
		rrCancel()

		if rrErr != nil {
			log.Warnf(ctx, "Reranker execution failed or timed out: %v, fallback to RRF results", rrErr)
		} else if len(rerankedCandidates) > 0 {
			newScoredChunks := make([]*retriever.ScoredChunk, len(rerankedCandidates))
			for i, rc := range rerankedCandidates {
				newScoredChunks[i] = &retriever.ScoredChunk{
					ChunkID:  rc.ID,
					DocID:    rc.DocID,
					ParentID: rc.ParentID,
					Score:    rc.Score,
					Content:  rc.Content,
					Metadata: rc.Metadata,
				}
			}
			scoredChunks = newScoredChunks
		}
	}

	// 5. 收集 Parent IDs 并从 MySQL 回查完整 Parent 粗粒度上下文
	parentIDs := make([]string, 0)
	for _, sc := range scoredChunks {
		if sc.ParentID != "" {
			parentIDs = append(parentIDs, sc.ParentID)
		}
	}

	parentMap, err := e.ragRepo.GetParentChunksByParentIDs(ctx, tenantID, parentIDs)
	if err != nil {
		log.Warnf(ctx, "Fetch parent chunks from mysql error: %v", err)
		parentMap = make(map[string]string)
	}

	// 5. 组装最相关结果集
	rawResults := make([]*RAGContext, 0, len(scoredChunks))
	for i, sc := range scoredChunks {
		if i >= topK {
			break
		}

		fullCtx := sc.Content
		if pText, ok := parentMap[sc.ParentID]; ok && pText != "" {
			fullCtx = pText
		}

		rawResults = append(rawResults, &RAGContext{
			ChunkID:      sc.ChunkID,
			DocID:        sc.DocID,
			ParentID:     sc.ParentID,
			ChildContent: sc.Content,
			FullContext:  fullCtx,
			Score:        sc.Score,
			Metadata:     sc.Metadata,
		})
	}

	// 6. 【生产级关键项】：首尾强化重排 (Sandwich Context Assembly)，解决 Lost in the Middle 效应！
	return retriever.ReorderSandwichContext(rawResults), nil
}
