# 生产级 RAG (Retrieval-Augmented Generation) 系统架构与设计方案

## 1. 架构目标与分层职责划分

根据生产架构权责设计，**`internal/data/` 目录专门服务于 MySQL / GORM 关系型数据库持久化**，向量数据库（Milvus / Qdrant 等）的存储、适配器与向量检索逻辑与 MySQL 解耦，归属于 RAG 业务层（`biz` 域）。

1. **动态可配置向量维度 (`EmbeddingConfig.Dimension`)**：
   - 彻底移除了代码中硬编码的 1536 维度逻辑。向量维度由 `configs/config.local.yaml` 的 `embedding.dimension`（或 `vector_db.milvus.dimension`）灵活配置注入，兼容不同模型（如 `bge-m3`: 1024, `text-embedding-3-small`: 1536, `text-embedding-3-large`: 3072）。
2. **Milvus Partition Key 级别多租户架构 (`store/milvus_adapter.go`)**：
   - 采用 Milvus 官方推荐的 **Partition Key 多租户模式**。在 Collection Schema 中将 `tenant_id` 声明为 `IsPartitionKey: true`。
   - **优势**：打破传统 4096 个分区的上限约束，Milvus 引擎通过哈希算法将海量租户数据均匀散列在内部分区中。检索时带有 `tenant_id == 'xxx'` 条件将自动触发 **分区裁剪 (Partition Pruning)**，无需全表扫库，检索性能相比普通标量过滤提升数倍！
3. **向量存储引擎物理重构 (`internal/biz/nocli/vector/store/`)**：
   - 包含 `vector_store.go` (高层抽象接口 `VectorStore`)、`milvus_adapter.go` (Milvus 驱动适配器) 与 `factory.go` (向量存储策略工厂 `NewVectorStore`)。
   - `internal/data/` 仅保留 MySQL `RAGRepo` 数据访问，符合清晰职责约束。
4. **Protobuf 规范定义 (`api/nocli/v1/knowledge.proto`)**：
   - 遵照项目规范，除 multipart 文件上传接口做特殊 HTTP 处理外，所有知识库管理接口（`CreateKnowledgeBase`, `ListKnowledgeBases`, `DeleteKnowledgeBase`）统一在 `api/nocli/v1/knowledge.proto` 中定义，并通过 `make api` 自动生成 HTTP & gRPC 服务存根。
5. **知识库与文件隔离架构 (Multi-Tenant KB Isolation)**：
   - 新增 `knowledge_bases` 知识库主表（关联 `KBID`、`UserID`、`TenantID` 与 `IsDefault`）。
   - **系统默认公共知识库**（`is_default = true`，如官方文档）与 **用户新增的自定义知识库**（`is_default = false`）严格物理与逻辑隔离，用户上传新文件不会影响默认公共知识库。
6. **两阶段语义感知动态切片算子 (`semantic_chunker.go`)**：在 `VectorEngine` 中默认开启两阶段语义切片：先通过递归字符切分生成 512 字符的原子底包，计算相邻底包的余弦相似度；高相似度（$\ge 0.75$）自动融合大包，极低相似度（$< 0.45$）识别为话题断层强行切割，兼顾处理速度与语义完整性。
7. **Embedding 与 Rerank 双模组支持直连 Ollama 本地引擎**：在 `configs/config.local.yaml` 中，Embedding 与 Rerank 均通过 `base_url: "http://localhost:11434/v1"` 直连 Ollama 本地加载的模型（如 `bge-m3` 与 `bge-reranker-large`）。

---

## 2. 本地配置文件范例 (`configs/config.local.yaml`)

```yaml
source:
  openai:
    api_key: "uVOsMuo_LHB_..."
    base_url: "https://api.modelarts-maas.com/openai/v1"
    model: "deepseek-v3.2"

  vector_db:
    driver: "milvus" # 支持 milvus | qdrant | pgvector
    milvus:
      address: "localhost:19530"
      db_name: "default"
      collection_name: "rag_knowledge"
      dimension: 1024                      # 向量索引生成维度 (通过配置传入)

  rag:
    knowledge_dir: "./workspace/knowledge" # 默认公共知识库系统扫描目录
    upload_dir: "./workspace/uploads"       # 用户自主上传文件的存储目录
    auto_reload: true                      # 启动时是否自动扫描重载知识库
    top_k: 5                               # 默认召回 TopK
    score_threshold: 0.6                   # 默认相似度得分阈值
    embedding:                             # 直连 Ollama 本地向量模型配置
      api_key: "ollama"
      base_url: "http://localhost:11434/v1" # Ollama 本地 API 接口地址
      model: "bge-m3"                       # Ollama 挂载的向量模型
      dimension: 1024                       # 向量维度配置 (如 1024 / 1536 / 768 / 384)
    rerank:                                # 直连 Ollama 本地重排模型配置
      enable: true
      driver: "llm"                        # 使用 OpenAI 兼容模式直连 Ollama 本地服务
      api_key: "ollama"
      base_url: "http://localhost:11434/v1" # Ollama 本地服务接口地址
      model: "bge-reranker-large"           # Ollama 挂载的重排模型名称
      timeout: 2000                         # 重排硬超时 (2000ms)
    chunker:                               # 细粒度与语义动态切片参数
      parent_size: 1500                    # Parent 块上限字符数
      child_size: 512                      # 底包字符数
      overlap: 30
      merge_threshold: 0.75                # 语义相似度合并阈值 (默认 0.75)
      split_threshold: 0.45                # 话题断层切割阈值 (默认 0.45)
```
