package parser

import (
	"context"
	"io"
)

// ParsedDocument 解析后的文档统一数据模型
type ParsedDocument struct {
	Title      string            `json:"title"`       // 文档标题
	SourceType string            `json:"source_type"` // 拓展名类型
	Content    string            `json:"content"`     // 格式化/扁平化后的纯文本内容
	Metadata   map[string]string `json:"metadata"`    // 解析提取的特定元数据
}

// DocumentParser 不同文件格式解析器的策略接口 (Strategy Pattern)
type DocumentParser interface {
	Parse(ctx context.Context, r io.Reader, filename string) (*ParsedDocument, error)
}
