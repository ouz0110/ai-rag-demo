# Go 生产级 RAG 系统完整落地方案

---

## 一、整体架构

```
                    用户 Query
                        │
                        ▼
              ┌─────────────────┐
              │  API Gateway    │  JWT 鉴权，提取用户角色/租户
              └────────┬────────┘
                       ▼
              ┌─────────────────┐
              │  Query Pipeline │  改写拆解、权限过滤、混合检索、RRF 融合、Rerank
              └────────┬────────┘
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
    ┌──────────┐ ┌──────────┐ ┌──────────┐
    │  Milvus  │ │  MySQL   │ │  Redis   │
    │ 向量检索  │ │ 元数据   │ │ 语义缓存  │
    └──────────┘ └──────────┘ └──────────┘
          │            │
          └─────┬──────┘
                ▼
       ┌──────────────┐
       │    LLM       │
       └──────────────┘
```

**存储分工原则**：Milvus 管向量检索，MySQL 管结构化元数据和上下文原文，Redis 管高频缓存。

---

## 二、分块策略：三层结构

```
原始 Markdown 文档
        │
        ▼
┌─────────────────────────────────────────────┐
│  一级：文档全文                              │
│  存储：MySQL documents.md_content            │
│  用途：按行号扩展上下文、重新分块时用          │
└─────────────────────────────────────────────┘
        │
        ▼  按 ## 或 ### 切分
┌─────────────────────────────────────────────┐
│  二级：章节父块（完整内容）                    │
│  存储：MySQL parent_chunks                   │
│  字段：parent_chunk_id, doc_id, h1, h2,      │
│         content(完整章节原文), start_line,     │
│         end_line, chunk_index                │
│  用途：检索命中后提供完整上下文给 LLM          │
└─────────────────────────────────────────────┘
        │
        ▼  按段落边界（空行）二次切分
┌─────────────────────────────────────────────┐
│  三级：段落子块（检索单元）                    │
│  内容：Milvus (text + embedding)             │
│  元数据：MySQL child_chunks                  │
│  字段：child_chunk_id, parent_chunk_id,      │
│         doc_id, h1, h2, h3, start_line,      │
│         end_line, has_table, has_code        │
│  用途：直接参与向量/BM25 检索                 │
└─────────────────────────────────────────────┘
```

---

## 三、分块核心规则

### 3.1 不可切割单元
- **表格**：识别后转为自然语言段落，标记 `has_table=true`，整个作为一个三级块，**绝不从中切断**
- **代码块**：识别 ` ``` ` 围栏，整体保护，**宁超长不切断**
- **列表**：连续的有序/无序列表视为一个整体

### 3.2 三级块长度控制
- **标准大小**：300~800 字符
- **最小阈值**：短于 50 字符的碎片块，向前合并到相邻块，禁止独立存在
- **超长处理**：超过 800 字符的段落，在句子边界（句号/问号/感叹号）处切断
- **强制继承**：所有三级块必须继承所属二级块的 h1、h2 和 parent_chunk_id

### 3.3 元数据注入

三级块入库时，**强制硬编码**元数据前缀到 Embedding 文本中：

```
[来源：{文件名} | 章节：{h1} > {h2} | 部门：{department} | 年份：{year}]
{段落正文}
```

权限字段（`permission_roles`、`department`）同时作为 Milvus 标量字段，用于 Pre-Filtering。

---

## 四、数据库表设计

### 4.1 MySQL 表

```sql
-- 文档表（一级）
CREATE TABLE documents (
    doc_id      VARCHAR(64) PRIMARY KEY,
    file_name   VARCHAR(512) NOT NULL,
    file_type   VARCHAR(32) NOT NULL,       -- pdf/docx/xlsx/md
    md_content  MEDIUMTEXT NOT NULL,        -- 完整 Markdown 原文
    department  VARCHAR(128),
    permission_roles JSON,                   -- ["HR","Finance","All"]
    year        INT,
    status      VARCHAR(32) DEFAULT 'processing', -- processing/ready/failed
    created_at  DATETIME DEFAULT NOW(),
    updated_at  DATETIME DEFAULT NOW()
);

-- 二级块表（父块，存完整章节内容）
CREATE TABLE parent_chunks (
    parent_chunk_id VARCHAR(64) PRIMARY KEY,
    doc_id          VARCHAR(64) NOT NULL,
    h1              VARCHAR(512),
    h2              VARCHAR(512),
    content         MEDIUMTEXT NOT NULL,     -- 章节完整原文
    chunk_index     INT NOT NULL,
    start_line      INT NOT NULL,
    end_line        INT NOT NULL,
    FOREIGN KEY (doc_id) REFERENCES documents(doc_id)
);

-- 三级块表（子块，仅存元数据）
CREATE TABLE child_chunks (
    child_chunk_id  VARCHAR(64) PRIMARY KEY,
    parent_chunk_id VARCHAR(64) NOT NULL,
    doc_id          VARCHAR(64) NOT NULL,
    h1              VARCHAR(512),
    h2              VARCHAR(512),
    h3              VARCHAR(512),
    start_line      INT NOT NULL,
    end_line        INT NOT NULL,
    has_table       BOOLEAN DEFAULT FALSE,
    has_code        BOOLEAN DEFAULT FALSE,
    FOREIGN KEY (parent_chunk_id) REFERENCES parent_chunks(parent_chunk_id),
    FOREIGN KEY (doc_id) REFERENCES documents(doc_id)
);
```

### 4.2 Milvus Collection

```go
// Milvus 字段定义
type MilvusChunk struct {
    ChildChunkID   string    `milvus:"child_chunk_id"`   // PK，与 MySQL 关联
    DocID          string    `milvus:"doc_id"`           // 过滤字段
    Text           string    `milvus:"text"`             // 带元数据前缀的文本
    Embedding      []float32 `milvus:"embedding"`        // 1536 维向量
    PermissionRoles []string `milvus:"permission_roles"` // Pre-Filtering 字段
    Department     string    `milvus:"department"`
    Year           int       `milvus:"year"`
}
```

### 4.3 Redis 缓存

```
Key: cache:query:{query_md5_hash}
Value: {序列化的检索结果 JSON}
TTL: 3600s（高频 FAQ 可延长至 24h）
```

---

## 五、检索完整流程

```
用户 Query: "研发部 P6 级别住宿费标准？"
    │
    ▼
[1. API Gateway]
   → JWT 鉴权，提取 user_roles: ["研发部", "Employee"]
    │
    ▼
[2. 查询 Redis 语义缓存]
   → 命中？直接返回（跳过后续步骤）
   → 未命中？继续
    │
    ▼
[3. Query 改写/拆解]（并发生成多个变体）
   → "P6 住宿费上限"
   → "P6 级别酒店报销标准"
    │
    ▼
[4. 混合检索]（并发执行）
   ├── BM25 稀疏检索（Gojieba + 业务词典）
   └── Dense 向量检索（Milvus）
       Filter: permission_roles IN ["研发部", "All"]
       Top-K: 20
    │
    ▼
[5. RRF 融合] k=60
   → 融合 BM25 和 Dense 排名，产出 Top-20
    │
    ▼
[6. Rerank 精排] 1.5s 硬超时
   → 成功？用精排结果 Top-5
   → 超时？降级取 RRF Top-5
    │
    ▼
[7. 上下文组装]
   → 根据 child_chunk_id 查 MySQL child_chunks
   → 取 parent_chunk_id，查 MySQL parent_chunks.content
   → 取相邻子块（chunk_index ± 1）
   → 拼装：父块完整内容 + 相邻块补充
   → 结构：[标题路径] + [完整父块内容] + [引用标记]
    │
    ▼
[8. Prompt 构造]
   System: "你是严谨的客服助手。仅基于【参考信息】回答。
            如果信息不足，回复'知识库未包含该信息'，禁止编造。"
   Context: 上一步组装的父块内容
   Question: 原始用户问题
    │
    ▼
[9. LLM 流式生成]（SSE 推送给前端）
    │
    ▼
[10. 结果写入 Redis 缓存]
    │
    ▼
   返回给用户（前端展示引用来源）
```

---

## 六、索引构建：Goroutine Worker Pool

```go
// 核心参数配置
type IndexConfig struct {
    Workers      int           // Worker 数量，建议 CPU 核数 * 2
    BatchSize    int           // 每批 Embedding 数量，建议 20-50
    RPS          int           // 对 Embedding API 的每秒请求限制
    RetryMax     int           // 单批失败重试次数
    RetryBackoff time.Duration // 重试退避时间
}

// Worker Pool 关键组件
// 1. 令牌桶限流：golang.org/x/time/rate
// 2. 任务投递：buffered channel
// 3. 错误处理：首个致命错误即中断
// 4. 优雅退出：context.WithCancel + sync.WaitGroup
```

---

## 七、容灾与降级策略

| 环节 | 故障 | 降级策略 |
|---|---|---|
| BM25 服务 | 不可用 | 仅用 Dense 检索结果 |
| Milvus | 超时 | 回退 MySQL 全文索引 |
| Rerank | 超时 1.5s | 直接用 RRF Top-5 |
| LLM | 超时 30s | 返回“系统繁忙，请稍后重试” |
| Embedding API | 限流 429 | 指数退避重试，最大 3 次 |

---

## 八、文档更新流程（软删除）

```
1. 收到更新请求
2. 将旧文档的 documents.status 改为 'deprecated'
3. MySQL 旧 parent_chunks 和 child_chunks 标记 is_deleted = true
4. Milvus 对应向量标记 is_deleted = true（标量字段过滤）
5. 新文档正常走 ETL + 索引流程
6. 深夜 Cron 任务：物理清理 7 天前的 is_deleted 数据
```

---

## 九、质量保障 Checklist

- [ ] BM25 分词配置业务自定义词典（产品型号、专有名词）
- [ ] 表格和代码块在分块时不可切割
- [ ] Embedding 文本强制注入元数据前缀
- [ ] 权限 Pre-Filtering 在 Milvus 检索时强制执行
- [ ] Rerank 硬超时降级兜底
- [ ] Prompt 强约束“找不到即说未找到”
- [ ] 三级碎片块（<50 字符）合并处理
- [ ] 文档全文存 MySQL，支持按行号扩展
- [ ] 父块存完整章节内容，子块命中后返回父块
- [ ] 软删除机制，避免物理删除引发磁盘碎片
- [ ] 高频查询 Redis 语义缓存
- [ ] LLM 输出绑定 chunk_id，前端可溯源
- [ ] 线上点踩样本自动收集至分析队列
- [ ] P99 全链路延迟 < 5s 监控告警
- [ ] 对外 API 配置计费熔断