package parser

import (
	"path/filepath"
	"strings"
)

// ParserFactory 文档解析策略工厂 (Factory Pattern)
type ParserFactory struct {
	parsers       map[string]DocumentParser // 文件扩展名与对应解析器映射表
	defaultParser DocumentParser            // 默认文本兜底解析器
}

func NewParserFactory() *ParserFactory {
	txtParser := NewTextParser()
	mdParser := NewMarkdownParser()
	csvParser := NewCSVParser()
	jsonParser := NewJSONParser()

	return &ParserFactory{
		parsers: map[string]DocumentParser{
			"txt":      txtParser,
			"md":       mdParser,
			"markdown": mdParser,
			"csv":      csvParser,
			"tsv":      csvParser,
			"json":     jsonParser,
		},
		defaultParser: txtParser,
	}
}

// GetParser 根据扩展名返回对应的 DocumentParser 解析器
func (f *ParserFactory) GetParser(filename string) DocumentParser {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if p, ok := f.parsers[ext]; ok {
		return p
	}
	return f.defaultParser
}
