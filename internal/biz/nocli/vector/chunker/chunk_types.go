package chunker

import (
	"fmt"
	"strings"
)

// ChunkUnit 切片原子单元模型
type ChunkUnit struct {
	ChunkID    string // 切片唯一 ID (UUID)
	ParentID   string // 父切片 ID (若本身为 Parent，则为空字符串)
	H1         string // 一级标题
	H2         string // 二级标题
	H3         string // 三级标题
	StartLine  int32  // 起始行号
	EndLine    int32  // 结束行号
	HasTable   int32  // 是否包含表格
	HasCode    int32  // 是否包含代码块
	ChunkType  string // 切片类型 (parent, text, table, code)
	Content    string // 文本内容
	ChunkIndex int32  // 切片全局顺序索引
	TokenCount int32  // 字符/Token 数
	IsParent   bool   // 是否为粗粒度父切片
}

// ParentChildResult 父子切片分割结果容器
type ParentChildResult struct {
	ParentChunks []*ChunkUnit // 粗粒度父切片列表 (上下文补全用)
	ChildChunks  []*ChunkUnit // 细粒度子切片列表 (向量化检索用)
}

// InjectMetadataPrefix 前缀元数据硬编码注入，大幅降低大模型幻觉率
func InjectMetadataPrefix(docTitle string, rawText string) string {
	rawText = strings.TrimSpace(rawText)
	if docTitle == "" || strings.HasPrefix(rawText, "[来源文档:") || strings.HasPrefix(rawText, "[来源:") {
		return rawText
	}
	return fmt.Sprintf("[来源文档：%s]\n%s", docTitle, rawText)
}

// InjectMetadataPrefixWithHierarchy 带层级面包屑的前缀元数据硬编码注入
func InjectMetadataPrefixWithHierarchy(docTitle, titlePath, rawText string) string {
	rawText = strings.TrimSpace(rawText)
	if strings.HasPrefix(rawText, "[来源:") || strings.HasPrefix(rawText, "[来源文档:") {
		return rawText
	}
	pathInfo := docTitle
	if titlePath != "" && titlePath != docTitle {
		pathInfo = fmt.Sprintf("%s > %s", docTitle, titlePath)
	}
	return fmt.Sprintf("[来源：%s]\n%s", pathInfo, rawText)
}
