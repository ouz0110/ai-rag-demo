package chunker

import (
	"fmt"
	"strings"
)

// ChunkUnit 切片原子单元模型
type ChunkUnit struct {
	ChunkID    string // 切片唯一 ID (UUID)
	ParentID   string // 父切片 ID (若本身为 Parent，则为空字符串)
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
	if docTitle == "" || strings.HasPrefix(rawText, "[来源文档:") {
		return rawText
	}
	return fmt.Sprintf("[来源文档：%s]\n%s", docTitle, rawText)
}
