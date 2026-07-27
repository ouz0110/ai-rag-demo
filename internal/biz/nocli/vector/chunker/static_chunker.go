package chunker

import (
	"fmt"
	"strings"

	"ai-rag-demo/internal/conf"

	"github.com/google/uuid"
)

// ParentChildChunker 基于静态字符长度与重叠步长规则的切片算子 (作为降级/兜底用)
type ParentChildChunker struct {
	ParentChunkSize int // 粗粒度父切片大小 (默认 1000 字符)
	ChildChunkSize  int // 细粒度子切片大小 (默认 250 字符)
	OverlapSize     int // 相邻切片重叠步长 (默认 30 字符)
}

// NewStaticChunkerFromConfig 根据 Config 配置初始化静态父子块切片算子
func NewStaticChunkerFromConfig(cfg *conf.Config) *ParentChildChunker {
	pSize := 1000
	cSize := 250
	overlap := 30

	if cfg != nil && cfg.Source.RAG != nil && cfg.Source.RAG.Chunker != nil {
		if cfg.Source.RAG.Chunker.ParentSize > 0 {
			pSize = cfg.Source.RAG.Chunker.ParentSize
		}
		if cfg.Source.RAG.Chunker.ChildSize > 0 {
			cSize = cfg.Source.RAG.Chunker.ChildSize
		}
		if cfg.Source.RAG.Chunker.Overlap >= 0 {
			overlap = cfg.Source.RAG.Chunker.Overlap
		}
	}
	return NewStaticChunker(pSize, cSize, overlap)
}

// NewStaticChunker 手动参数初始化静态父子块切片算子
func NewStaticChunker(parentSize, childSize, overlap int) *ParentChildChunker {
	if parentSize <= 0 {
		parentSize = 1000
	}
	if childSize <= 0 {
		childSize = 250
	}
	if overlap < 0 {
		overlap = 30
	}
	return &ParentChildChunker{
		ParentChunkSize: parentSize,
		ChildChunkSize:  childSize,
		OverlapSize:     overlap,
	}
}

// Split 将纯文本按静态固定长度切割为 Parent-Child 结构化切片
func (c *ParentChildChunker) Split(text string) *ParentChildResult {
	text = strings.TrimSpace(text)
	if text == "" {
		return &ParentChildResult{}
	}

	runes := []rune(text)
	totalLen := len(runes)

	parentUnits := make([]*ChunkUnit, 0)
	childUnits := make([]*ChunkUnit, 0)

	parentIndex := 0
	childIndex := 0

	for i := 0; i < totalLen; {
		end := i + c.ParentChunkSize
		if end > totalLen {
			end = totalLen
		}

		parentContent := string(runes[i:end])
		parentID := fmt.Sprintf("p_%s", uuid.New().String())

		parentUnit := &ChunkUnit{
			ChunkID:    parentID,
			ParentID:   "",
			Content:    parentContent,
			ChunkIndex: int32(parentIndex),
			TokenCount: int32(len([]rune(parentContent))),
			IsParent:   true,
		}
		parentUnits = append(parentUnits, parentUnit)
		parentIndex++

		// 对 Parent 块进行 Child 细粒度二次切分
		parentRunes := []rune(parentContent)
		pLen := len(parentRunes)

		for j := 0; j < pLen; {
			cEnd := j + c.ChildChunkSize
			if cEnd > pLen {
				cEnd = pLen
			}

			childContent := string(parentRunes[j:cEnd])
			childID := fmt.Sprintf("c_%s", uuid.New().String())

			childUnit := &ChunkUnit{
				ChunkID:    childID,
				ParentID:   parentID,
				Content:    childContent,
				ChunkIndex: int32(childIndex),
				TokenCount: int32(len([]rune(childContent))),
				IsParent:   false,
			}
			childUnits = append(childUnits, childUnit)
			childIndex++

			if cEnd == pLen {
				break
			}
			j += c.ChildChunkSize - c.OverlapSize
		}

		if end == totalLen {
			break
		}
		i += c.ParentChunkSize - c.OverlapSize
	}

	return &ParentChildResult{
		ParentChunks: parentUnits,
		ChildChunks:  childUnits,
	}
}
