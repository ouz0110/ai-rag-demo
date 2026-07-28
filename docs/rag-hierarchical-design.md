# 生产级通用层级 RAG Parser & Chunker 架构优化方案

---

## 一、 背景与诊断

目前系统的向量检索与知识库解析切片引擎（`internal/biz/nocli/vector`）在实际生产运行中存在以下瓶颈：

1. **Parser 缺乏结构化提取**：现有 `parser.go` 仅包含 `ParsedDocument`（`Title`, `Content` 纯文本），解析文件时未保留 Markdown 的天然树状标题层级（`# H1`, `## H2`）、节点语法类型（Header, Paragraph, Table, CodeBlock）及行号范围。
2. **Chunker 粗暴滑动切分**：采用固定的字符窗口切分，没有针对 Markdown 格式的特殊处理，极易破坏表格（Table）、代码块（CodeBlock）等不可切割单元的完整性。
3. **父子块（Parent-Child）粒度未与语义对齐**：没有以完整的“章节/Section”作为父块，导致召回子块后回查到的父块缺失全局标题脉络与完整的章节逻辑。
4. **硬编码标题上下文前缀缺失**：向量化阶段缺少硬编码前缀注入（如 `[来源：xx.md | 章节：H1 > H2]`），影响向量索引的语义聚焦度和大模型理解。

---

## 二、 整体架构：三层结构模型

系统将按以下三层结构进行层级解构：

```
                        原始 Markdown / 异构文件
                                   │
                                   ▼
                       ┌──────────────────────┐
                       │   Document Parser    │ (解析为通用 AST 树)
                       └───────────┬──────────┘
                                   │
       ┌───────────────────────────┼───────────────────────────┐
       ▼                           ▼                           ▼
┌──────────────┐           ┌──────────────┐           ┌──────────────┐
│  一级：文档   │           │ 二级：章节父块│           │ 三级：段落子块│
│ (Doc Level)  │           │ (Parent/Sec) │           │ (Child/Leaf) │
└──────┬───────┘           └──────┬───────┘           └──────┬───────┘
       │                          │                          │
       ▼                          ▼                          ▼
 MySQL documents          MySQL parent_chunks        Milvus (Vector) &
(存储完整 Raw MD)         (存完整 H1/H2 章节)          MySQL child_chunks
```

1. **一级：文档全文 (Doc Level)**：存储于 MySQL `knowledge_documents` 表，保留完整 Raw Markdown 内容与 SHA256 哈希，用于全局 Trace、增量比较与按需重新分块。
2. **二级：章节父块 (Parent Section Level)**：根据 `# H1`, `## H2` 划分出完整业务逻辑章节，存储于 MySQL。**作为检索命中后提供给 LLM 的完整上下文**。
3. **三级：段落子块 (Child Leaf Level)**：在二级章节内部，按自然段落及不可切分单元（Table、CodeBlock）二次切分（控制在 300~800 字符），硬编码注入 `[来源 | 标题路径]` 前缀后存储于 Milvus 与 MySQL 细粒度表。**仅作为精细化向量/BM25 检索单元**。

---

## 三、 通用 Parser 节点树数据模型设计 (`parser/parser.go`)

为了实现 Parser 与 Chunker 的彻底解耦，统一支持 Markdown 及未来扩展的 PDF、Word、HTML、CSV 等格式，定义标准通用的 **Hierarchical Document AST** 架构模型：

```go
package parser

import (
	"context"
	"io"
)

// NodeType 表达通用的文档节点类型
type NodeType string

const (
	NodeRoot      NodeType = "root"       // 根节点（整个文档）
	NodeSection   NodeType = "section"    // 章节/标题组（父块容器）
	NodeHeading   NodeType = "heading"    // 标题节点
	NodeParagraph NodeType = "paragraph"  // 普通文本段落
	NodeTable     NodeType = "table"      // 表格节点
	NodeCodeBlock NodeType = "code_block" // 代码块节点
	NodeList      NodeType = "list"       // 列表节点
)

// DocNode 通用文档语法树节点 (Generic AST Node)
type DocNode struct {
	NodeID       string            `json:"node_id"`       // 节点唯一 UUID
	ParentID     string            `json:"parent_id"`     // 父节点 ID
	Type         NodeType          `json:"type"`          // 节点类型
	HeadingLevel int               `json:"heading_level"` // 标题层级 (1~6；非 Heading 为 0)
	Title        string            `json:"title"`         // 节点/章节标题
	Breadcrumbs  []string          `json:"breadcrumbs"`   // 上级标题路径 [H1, H2, H3]
	Content      string            `json:"content"`       // 节点纯文本/格式化内容
	StartLine    int               `json:"start_line"`    // 起始行号
	EndLine      int               `json:"end_line"`      // 结束行号
	StartPage    int               `json:"start_page"`    // 起始页码 (PDF/Word 扩展)
	EndPage      int               `json:"end_page"`      // 结束页码 (PDF/Word 扩展)
	Metadata     map[string]string `json:"metadata"`      // 扩展元数据 (如 language, has_table)
	Children     []*DocNode        `json:"children"`      // 多叉子节点树
}

// DocumentSection 标准二级逻辑块视图 (Section View)
type DocumentSection struct {
	SectionID   string     `json:"section_id"`   // 章节 ID (ParentID)
	H1          string     `json:"h1"`           // 归属 H1 标题
	H2          string     `json:"h2"`           // 归属 H2 标题
	H3          string     `json:"h3"`           // 归属 H3 标题
	TitlePath   string     `json:"title_path"`   // 完整层级路径字符串 (如 "核心规范 > 数据库 > 事务")
	FullContent string     `json:"full_content"` // 章节内所有子节点的完整汇总原文 (存入 MySQL 父块)
	StartLine   int        `json:"start_line"`
	EndLine     int        `json:"end_line"`
	StartPage   int        `json:"start_page"`
	EndPage     int        `json:"end_page"`
	LeafNodes   []*DocNode `json:"leaf_nodes"`   // 章节内部包含的叶子节点列表 (供三级切片使用)
}

// ParsedDocument 策略解析器的标准统一返回值
type ParsedDocument struct {
	DocID       string             `json:"doc_id"`       // 文档 ID
	Title       string             `json:"title"`        // 文档总标题
	SourceType  string             `json:"source_type"`  // 文件扩展名 (md, pdf, docx, csv 等)
	RawContent  string             `json:"raw_content"`  // 全文原始字符串 (一级块)
	Root        *DocNode           `json:"root"`         // 全局文档 AST 语法树根节点
	Sections    []*DocumentSection `json:"sections"`     // 提取拉平后的二级章节父块列表
	GlobalMeta  map[string]string  `json:"global_meta"`  // 文档全局属性
}

// DocumentParser 异构文件格式解析器的通用策略接口
type DocumentParser interface {
	Parse(ctx context.Context, r io.Reader, filename string) (*ParsedDocument, error)
}
```

---

## 四、 切分逻辑与不可切割单元保护

Chunker 读取 `ParsedDocument` 后，执行如下切片保护逻辑：

1. **表格 (`NodeTable`)**：整块保留，标记 `HasTable = 1`，**绝不从中间截断**。
2. **代码块 (`NodeCodeBlock`)**：保留完整 ` ``` ` 围栏，标记 `HasCode = 1`，**宁可字符略超长也不截断**。
3. **列表 (`NodeList`)**：连续列表项合并作为一个整段 Child Chunk。
4. **超短碎片 (< 50 字符)**：自动向前合并到同一 Section 内的前一个 Child Chunk。
5. **元数据硬编码前缀注入**：
   在写入向量库之前，给所有 Child Chunk 强制添加文本头部前缀：
   ```text
   [来源：{文件名} | 章节：{H1} > {H2} > {H3}]
   {正文内容}
   ```

---

## 五、 数据库模型与检索 Pipeline

### 1. MySQL 模型扩展 (`knowledge_chunks`)
在 `KnowledgeChunkModel` 中增加以下结构化字段：
- `H1`, `H2`, `H3`: 记录层级标题
- `StartLine`, `EndLine`: 记录物理行号范围
- `HasTable`, `HasCode`: 标志位

### 2. 召回与上下文还原流程
1. 向量 / BM25 检索 Milvus，获取相关性较高的 Child Chunk ID 列表及对应的 `ParentID`。
2. 根据 `ParentID` 批量检索 MySQL `knowledge_chunks` 拿到完整的二级章节父块 `FullContent`。
3. 执行首尾强化重排 (Sandwich Context Assembly)，替换为整章父块内容输出给 LLM。
