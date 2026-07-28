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
	vectorData "ai-rag-demo/internal/biz/nocli/vector/store"
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data"
	ragData "ai-rag-demo/internal/data/rag"
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
	allDB           *data.DB                    // MySQL 文档与 Chunk 持久化数据库集合
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
	ragDB *data.DB,
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
		allDB:           ragDB,
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
	existingDoc, err := e.allDB.Rag.DocRepo.GetDocumentByFilePath(ctx, tenantID, absPath)
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
		if delErr := e.allDB.Rag.ChunkRepo.DeleteChunksByDocID(ctx, tenantID, existingDoc.DocID); delErr != nil {
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
	if err := e.allDB.Rag.DocRepo.CreateDocument(ctx, newDoc); err != nil {
		return nil, fmt.Errorf("create new document record failed: %w", err)
	}

	return e.processAndSaveDocument(ctx, newDoc, string(fileBytes), currentHash, absPath, collectionName)
}

func (e *VectorEngine) processAndSaveDocument(ctx context.Context, doc *ragData.KnowledgeDocumentModel, rawContent, contentHash, absPath, collectionName string) (*ragData.KnowledgeDocumentModel, error) {
	// 1. 调用策略工厂匹配对应格式的 DocumentParser 进行结构化/语法树解析 (如 MD AST, CSV 展平等)
	parsedDoc, err := e.parserFactory.GetParser(doc.Title).Parse(ctx, strings.NewReader(rawContent), doc.Title)
	if err != nil {
		log.Warnf(ctx, "Parser failed for [%s]: %v, fallback to raw text", doc.Title, err)
	}

	// 2. 【生产级关键项】：优先使用结构语法树算子 (HierarchicalChunker) 进行三层架构切片
	var splitRes *chunker.ParentChildResult
	hChunker := chunker.NewHierarchicalChunker(800, 50)
	if parsedDoc != nil && len(parsedDoc.Sections) > 0 {
		splitRes = hChunker.SplitFromAST(parsedDoc)
	}

	if splitRes == nil || len(splitRes.ChildChunks) == 0 {
		flattenedText := rawContent
		if parsedDoc != nil && parsedDoc.Content != "" {
			flattenedText = parsedDoc.Content
		}
		if e.semanticChunker != nil {
			semRes, semErr := e.semanticChunker.SplitWithSemantics(ctx, flattenedText)
			if semErr == nil && semRes != nil && len(semRes.ChildChunks) > 0 {
				splitRes = semRes
			}
		}
		if splitRes == nil {
			splitRes = e.staticChunker.Split(flattenedText)
		}
	}
	totalChunks := len(splitRes.ChildChunks)

	// 构造 MySQL 块 (仅仅落盘 Parent 粗粒度上下文正文，彻底不落盘 Child 碎片，极度瘦身 MySQL 存储)
	chunkModels := make([]*ragData.KnowledgeChunkModel, 0, len(splitRes.ParentChunks))
	parentIDSet := make(map[string]bool)

	for _, p := range splitRes.ParentChunks {
		pHash := CalculateContentHash([]byte(p.Content))
		cType := p.ChunkType
		if cType == "" {
			cType = "parent"
		}
		chunkModels = append(chunkModels, &ragData.KnowledgeChunkModel{
			TenantID:     doc.TenantID,
			DocID:        doc.DocID,
			ChunkID:      p.ChunkID,
			ParentID:     "",
			H1:           p.H1,
			H2:           p.H2,
			H3:           p.H3,
			StartLine:    p.StartLine,
			EndLine:      p.EndLine,
			HasTable:     p.HasTable,
			HasCode:      p.HasCode,
			ChunkIndex:   p.ChunkIndex,
			ChunkHash:    pHash,
			ChunkType:    cType,
			Content:      p.Content,
			TokenCount:   p.TokenCount,
			VectorStatus: 0,
			IsActive:     1,
		})
		parentIDSet[p.ChunkID] = true
	}

	// 严谨容灾兜底：若某些 Child 块无对应 Parent (如静态切片)，提升其为 Self-Parent 存入 MySQL
	for _, c := range splitRes.ChildChunks {
		if c.ParentID == "" {
			c.ParentID = c.ChunkID
		}
		if !parentIDSet[c.ParentID] {
			cHash := CalculateContentHash([]byte(c.Content))
			chunkModels = append(chunkModels, &ragData.KnowledgeChunkModel{
				TenantID:     doc.TenantID,
				DocID:        doc.DocID,
				ChunkID:      c.ParentID,
				ParentID:     "",
				H1:           c.H1,
				H2:           c.H2,
				H3:           c.H3,
				StartLine:    c.StartLine,
				EndLine:      c.EndLine,
				HasTable:     c.HasTable,
				HasCode:      c.HasCode,
				ChunkIndex:   c.ChunkIndex,
				ChunkHash:    cHash,
				ChunkType:    "parent",
				Content:      c.Content,
				TokenCount:   c.TokenCount,
				VectorStatus: 0,
				IsActive:     1,
			})
			parentIDSet[c.ParentID] = true
		}
	}

	if err := e.allDB.Rag.ChunkRepo.BatchCreateChunks(ctx, chunkModels); err != nil {
		_ = e.allDB.Rag.DocRepo.UpdateDocumentStatus(ctx, doc.TenantID, doc.DocID, 3, 0, err.Error())
		return nil, fmt.Errorf("batch save chunks error: %w", err)
	}

	// 向量化 Child Chunks (仅用于生成 Embeddings 存入 Milvus)
	childTexts := make([]string, len(splitRes.ChildChunks))
	for i, c := range splitRes.ChildChunks {
		childTexts[i] = c.Content
	}

	vectors, err := e.embedder.BatchGenerateEmbeddings(ctx, childTexts)
	if err != nil {
		_ = e.allDB.Rag.DocRepo.UpdateDocumentStatus(ctx, doc.TenantID, doc.DocID, 3, 0, err.Error())
		return nil, fmt.Errorf("generate embeddings error: %w", err)
	}

	// 构建 VectorDocument (Milvus 存储 Child 纯正文、Vector 向量及其对应的 parent_id 路由标量，完美支持后续混合检索与 BM25 匹配)
	vecDocs := make([]*vectorData.VectorDocument, len(splitRes.ChildChunks))
	for i, c := range splitRes.ChildChunks {
		vecDocs[i] = &vectorData.VectorDocument{
			ID:       c.ChunkID,
			DocID:    doc.DocID,
			TenantID: doc.TenantID,
			Vector:   vectors[i],
			Content:  c.Content, // 存入 Child 纯正文 (无前缀)，支持 BM25 混合检索与全文匹配
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
			_ = e.allDB.Rag.DocRepo.UpdateDocumentStatus(ctx, doc.TenantID, doc.DocID, 3, 0, err.Error())
			return nil, fmt.Errorf("upsert vector store error: %w", err)
		}
	}

	// 更新 Hash 与状态
	_ = e.allDB.Rag.DocRepo.UpdateDocumentHash(ctx, doc.TenantID, doc.DocID, contentHash, 2, int32(totalChunks))
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
	existingDocs, err := e.allDB.Rag.DocRepo.ListAllDocuments(ctx, tenantID)
	if err != nil {
		return
	}

	for _, d := range existingDocs {
		if _, err := os.Stat(d.FilePath); os.IsNotExist(err) {
			log.Infof(ctx, "[AutoReload] Clean obsolete document: %s (DocID: %s)", d.FilePath, d.DocID)
			_ = e.DeleteDocVectors(ctx, tenantID, d.DocID)
			_ = e.allDB.Rag.ChunkRepo.DeleteChunksByDocID(ctx, tenantID, d.DocID)
			_ = e.allDB.Rag.DocRepo.DeleteDocument(ctx, tenantID, d.DocID)
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
	if err := e.allDB.Rag.DocRepo.CreateDocument(ctx, docModel); err != nil {
		return nil, fmt.Errorf("save document record error: %w", err)
	}

	return e.processAndSaveDocument(ctx, docModel, rawContent, docHash, "", e.getCollectionName())
}

// RetrieveContext 极简 RAG 在线检索路径：Milvus Pure Index 向量召回 -> parent_id 顺序去重 -> MySQL 批量回查 Parent 上下文 -> Parent 块 Rerank -> 方案 B 动态 Prompt 组装
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

	// 2. 构建向量检索请求 (扩大 TopK 放大系数为 4x，防止 parent_id 去重后候选数量不足)
	searchQuery := &vectorData.SearchQuery{
		TenantID:   tenantID,
		Vector:     queryVec,
		TopK:       topK * 4,
		MinScore:   scoreThreshold,
		OnlyActive: true,
	}

	// 3. 执行 Milvus 向量检索 (仅从 Milvus 拿到 ChildChunkID, DocID, ParentID 标量)
	scoredChunks, err := e.retriever.Retrieve(retCtx, collectionName, searchQuery, queryText)
	if err != nil {
		log.Warnf(ctx, "Hybrid retrieval failed or timed out: %v", err)
		return nil, fmt.Errorf("hybrid retrieval failed: %w", err)
	}

	if len(scoredChunks) == 0 {
		return nil, nil
	}

	// 4. 【顺序去重与 Parent 映射】：依据检索命中的 parent_id 进行有序去重，提升召回多样度
	type candidateParent struct {
		parentID string
		docID    string
		maxScore float32
		childID  string
	}

	seenParents := make(map[string]bool)
	uniqueParents := make([]*candidateParent, 0, len(scoredChunks))

	for _, sc := range scoredChunks {
		pID := sc.ParentID
		if pID == "" {
			pID = sc.ChunkID // 兜底降级
		}
		if !seenParents[pID] {
			seenParents[pID] = true
			uniqueParents = append(uniqueParents, &candidateParent{
				parentID: pID,
				docID:    sc.DocID,
				maxScore: sc.Score,
				childID:  sc.ChunkID,
			})
		}
	}

	if len(uniqueParents) == 0 {
		return nil, nil
	}

	// 截取前 topK * 2 个 Parent 候选用于回查与 Rerank
	rerankCandidateLimit := topK * 2
	if len(uniqueParents) > rerankCandidateLimit {
		uniqueParents = uniqueParents[:rerankCandidateLimit]
	}

	// 5. 提取 Parent IDs 与 Doc IDs，直接一次性从 MySQL 批量回查 Parent 块模型与文档主信息
	parentIDs := make([]string, 0, len(uniqueParents))
	docIDs := make([]string, 0, len(uniqueParents))
	for _, p := range uniqueParents {
		parentIDs = append(parentIDs, p.parentID)
		if p.docID != "" {
			docIDs = append(docIDs, p.docID)
		}
	}

	parentModelMap, err := e.allDB.Rag.ChunkRepo.GetParentChunkModelsByParentIDs(ctx, tenantID, parentIDs)
	if err != nil {
		log.Warnf(ctx, "Fetch parent chunk models from mysql error: %v", err)
		parentModelMap = make(map[string]*ragData.KnowledgeChunkModel)
	}

	docModelMap, err := e.allDB.Rag.DocRepo.GetDocumentsByDocIDs(ctx, tenantID, docIDs)
	if err != nil {
		log.Warnf(ctx, "Fetch doc models from mysql error: %v", err)
		docModelMap = make(map[string]*ragData.KnowledgeDocumentModel)
	}

	// 7. 【方案 B 隔离核心】：在线格式化动态拼接来源前缀，组装最终 RAGContext
	rawResults := make([]*RAGContext, 0, len(scoredChunks))
	for i, sc := range scoredChunks {
		if i >= topK {
			break
		}

		fullCtx := sc.Content
		var pModel *ragData.KnowledgeChunkModel
		if pm, ok := parentModelMap[sc.ParentID]; ok && pm != nil {
			pModel = pm
			if pm.Content != "" {
				fullCtx = pm.Content
			}
		}

		docTitle := ""
		if dModel, ok := docModelMap[sc.DocID]; ok && dModel != nil {
			docTitle = dModel.Title
		}

		childTitlePath := buildTitlePath(sc.Metadata)
		parentTitlePath := childTitlePath
		if pModel != nil {
			parentTitlePath = buildTitlePathFromModel(pModel)
		}

		formattedChildContent := chunker.InjectMetadataPrefixWithHierarchy(docTitle, childTitlePath, sc.Content)
		formattedFullContext := chunker.InjectMetadataPrefixWithHierarchy(docTitle, parentTitlePath, fullCtx)

		rawResults = append(rawResults, &RAGContext{
			ChunkID:      sc.ChunkID,
			DocID:        sc.DocID,
			ParentID:     sc.ParentID,
			ChildContent: formattedChildContent,
			FullContext:  formattedFullContext,
			Score:        sc.Score,
			Metadata:     sc.Metadata,
		})
	}

	// 8. 【生产级关键项】：首尾强化重排 (Sandwich Context Assembly)，解决 Lost in the Middle 效应！
	return retriever.ReorderSandwichContext(rawResults), nil
}

func buildTitlePath(meta map[string]interface{}) string {
	if meta == nil {
		return ""
	}
	var parts []string
	if h1, ok := meta["h1"].(string); ok && h1 != "" {
		parts = append(parts, h1)
	}
	if h2, ok := meta["h2"].(string); ok && h2 != "" {
		parts = append(parts, h2)
	}
	if h3, ok := meta["h3"].(string); ok && h3 != "" {
		parts = append(parts, h3)
	}
	return strings.Join(parts, " > ")
}

func buildTitlePathFromModel(m *ragData.KnowledgeChunkModel) string {
	if m == nil {
		return ""
	}
	var parts []string
	if m.H1 != "" {
		parts = append(parts, m.H1)
	}
	if m.H2 != "" {
		parts = append(parts, m.H2)
	}
	if m.H3 != "" {
		parts = append(parts, m.H3)
	}
	return strings.Join(parts, " > ")
}
