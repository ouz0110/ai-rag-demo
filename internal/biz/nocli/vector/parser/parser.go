package parser

import (
	"context"
	"io"
)

// NodeType 表达通用的文档节点类型 (覆盖 MD, PDF, Word, HTML, CSV 等)
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
	NodeID       string            `json:"node_id"`        // 节点唯一 UUID
	ParentID     string            `json:"parent_id"`      // 父节点 ID
	Type         NodeType          `json:"type"`           // 节点类型
	HeadingLevel int               `json:"heading_level"`  // 标题层级 (1~6；非 Heading 为 0)
	Title        string            `json:"title"`          // 节点/章节标题
	Breadcrumbs  []string          `json:"breadcrumbs"`    // 上级标题路径 [H1, H2, H3]
	Content      string            `json:"content"`        // 节点纯文本/格式化内容
	StartLine    int               `json:"start_line"`     // 起始行号 (1-indexed)
	EndLine      int               `json:"end_line"`       // 结束行号
	StartPage    int               `json:"start_page"`     // 起始页码 (PDF/Word 扩展)
	EndPage      int               `json:"end_page"`       // 结束页码 (PDF/Word 扩展)
	Metadata     map[string]string `json:"metadata"`       // 扩展元数据 (如 language, has_table)
	Children     []*DocNode        `json:"children"`       // 多叉子节点树
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
	Content     string             `json:"content"`      // 兼顾全文本兼容字段 (即 RawContent)
	RawContent  string             `json:"raw_content"`  // 全文原始字符串 (一级块)
	Root        *DocNode           `json:"root"`         // 全局文档 AST 语法树根节点
	Sections    []*DocumentSection `json:"sections"`     // 提取拉平后的二级章节父块列表
	GlobalMeta  map[string]string  `json:"global_meta"`  // 文档全局属性
	Metadata    map[string]string  `json:"metadata"`     // 兼顾旧 API 元数据映射
}

// DocumentParser 不同文件格式解析器的策略接口 (Strategy Pattern)
type DocumentParser interface {
	Parse(ctx context.Context, r io.Reader, filename string) (*ParsedDocument, error)
}

