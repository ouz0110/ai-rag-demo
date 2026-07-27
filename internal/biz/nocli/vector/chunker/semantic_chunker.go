package chunker

import (
	"context"
	"fmt"
	"math"
	"strings"

	"ai-rag-demo/internal/biz/nocli/vector/embedder"
	"ai-rag-demo/internal/conf"

	"github.com/google/uuid"
)

// SemanticChunker 两阶段语义感知动态边界切片算子 (基于 512 字符底包 + 向量余弦相似度动态合并与话题断层切割)
type SemanticChunker struct {
	BaseChunkSize  int                // 原子底包目标字符数 (默认 512)
	MaxChunkSize   int                // 合并后 Parent 块上限字符数 (默认 1500)
	MergeThreshold float32            // 高相似度合并阈值 (默认 0.75)
	SplitThreshold float32            // 话题断层切割阈值 (默认 0.45)
	embedder       *embedder.Embedder // 向量 Embedding 计算器
}

// NewSemanticChunkerFromConfig 根据 Config 配置初始化语义感知动态切片算子
func NewSemanticChunkerFromConfig(cfg *conf.Config, emb *embedder.Embedder) *SemanticChunker {
	bSize := 512
	mSize := 1500
	mThreshold := float32(0.75)
	sThreshold := float32(0.45)

	if cfg != nil && cfg.Source.RAG != nil && cfg.Source.RAG.Chunker != nil {
		if cfg.Source.RAG.Chunker.ChildSize > 0 {
			bSize = cfg.Source.RAG.Chunker.ChildSize
		}
		if cfg.Source.RAG.Chunker.ParentSize > 0 {
			mSize = cfg.Source.RAG.Chunker.ParentSize
		}
		if cfg.Source.RAG.Chunker.MergeThreshold > 0 {
			mThreshold = cfg.Source.RAG.Chunker.MergeThreshold
		}
		if cfg.Source.RAG.Chunker.SplitThreshold > 0 {
			sThreshold = cfg.Source.RAG.Chunker.SplitThreshold
		}
	}

	return &SemanticChunker{
		BaseChunkSize:  bSize,
		MaxChunkSize:   mSize,
		MergeThreshold: mThreshold,
		SplitThreshold: sThreshold,
		embedder:       emb,
	}
}

// SplitWithSemantics 执行两阶段语义感知动态切片算法
func (c *SemanticChunker) SplitWithSemantics(ctx context.Context, text string) (*ParentChildResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return &ParentChildResult{}, nil
	}

	// 阶段 1：使用递归字符切分算法将长文档切成 512 字符左右的原子“底包”
	baseChunks := c.recursiveSplit(text, c.BaseChunkSize)
	if len(baseChunks) == 0 {
		return &ParentChildResult{}, nil
	}
	if len(baseChunks) == 1 {
		pID := fmt.Sprintf("p_%s", uuid.New().String())
		cID := fmt.Sprintf("c_%s", uuid.New().String())
		unitP := &ChunkUnit{ChunkID: pID, Content: baseChunks[0], TokenCount: int32(len([]rune(baseChunks[0]))), IsParent: true}
		unitC := &ChunkUnit{ChunkID: cID, ParentID: pID, Content: baseChunks[0], TokenCount: int32(len([]rune(baseChunks[0]))), IsParent: false}
		return &ParentChildResult{
			ParentChunks: []*ChunkUnit{unitP},
			ChildChunks:  []*ChunkUnit{unitC},
		}, nil
	}

	// 阶段 2：批量计算所有底包的 Embedding 向量
	vectors, err := c.embedder.BatchGenerateEmbeddings(ctx, baseChunks)
	if err != nil || len(vectors) != len(baseChunks) {
		// 若向量生成失败，降级回退到静态规则切分
		staticChunker := NewStaticChunker(c.MaxChunkSize, c.BaseChunkSize, 30)
		return staticChunker.Split(text), nil
	}

	// 阶段 3 & 4：计算相邻底包余弦相似度，高相似度合并，低相似度话题断层切割
	parentUnits := make([]*ChunkUnit, 0)
	childUnits := make([]*ChunkUnit, 0)

	var currentParentBuilder strings.Builder
	currentParentID := fmt.Sprintf("p_%s", uuid.New().String())
	parentIndex := 0
	childIndex := 0

	for i := 0; i < len(baseChunks); i++ {
		childID := fmt.Sprintf("c_%s", uuid.New().String())
		childUnit := &ChunkUnit{
			ChunkID:    childID,
			ParentID:   currentParentID,
			Content:    baseChunks[i],
			ChunkIndex: int32(childIndex),
			TokenCount: int32(len([]rune(baseChunks[i]))),
			IsParent:   false,
		}
		childUnits = append(childUnits, childUnit)
		childIndex++

		if currentParentBuilder.Len() > 0 {
			currentParentBuilder.WriteString("\n")
		}
		currentParentBuilder.WriteString(baseChunks[i])

		if i < len(baseChunks)-1 {
			sim := cosineSimilarity(vectors[i], vectors[i+1])
			nextLen := len([]rune(baseChunks[i+1]))
			currLen := len([]rune(currentParentBuilder.String()))

			shouldSplit := sim < c.SplitThreshold || (currLen+nextLen > c.MaxChunkSize)
			if !shouldSplit && sim < c.MergeThreshold && currLen >= c.BaseChunkSize {
				shouldSplit = true
			}

			if shouldSplit {
				parentUnit := &ChunkUnit{
					ChunkID:    currentParentID,
					ParentID:   "",
					Content:    currentParentBuilder.String(),
					ChunkIndex: int32(parentIndex),
					TokenCount: int32(len([]rune(currentParentBuilder.String()))),
					IsParent:   true,
				}
				parentUnits = append(parentUnits, parentUnit)
				parentIndex++

				currentParentBuilder.Reset()
				currentParentID = fmt.Sprintf("p_%s", uuid.New().String())
			}
		} else {
			parentUnit := &ChunkUnit{
				ChunkID:    currentParentID,
				ParentID:   "",
				Content:    currentParentBuilder.String(),
				ChunkIndex: int32(parentIndex),
				TokenCount: int32(len([]rune(currentParentBuilder.String()))),
				IsParent:   true,
			}
			parentUnits = append(parentUnits, parentUnit)
		}
	}

	return &ParentChildResult{
		ParentChunks: parentUnits,
		ChildChunks:  childUnits,
	}, nil
}

// recursiveSplit 递归字符切割算法 (分隔符优先级: 段落 > 句子 > 词/字)
func (c *SemanticChunker) recursiveSplit(text string, targetSize int) []string {
	separators := []string{"\n\n", "\n", "。", "！", "？", "；", ". ", "! ", "? ", " ", ""}
	return c.splitTextBySeparators(text, targetSize, separators)
}

func (c *SemanticChunker) splitTextBySeparators(text string, targetSize int, separators []string) []string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) <= targetSize || len(separators) == 0 {
		if text != "" {
			return []string{text}
		}
		return nil
	}

	sep := separators[0]
	nextSeparators := separators[1:]

	var splits []string
	if sep == "" {
		runes := []rune(text)
		for i := 0; i < len(runes); i += targetSize {
			end := i + targetSize
			if end > len(runes) {
				end = len(runes)
			}
			splits = append(splits, string(runes[i:end]))
		}
		return splits
	}

	parts := strings.Split(text, sep)
	var currentChunk strings.Builder

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if len([]rune(part)) > targetSize {
			if currentChunk.Len() > 0 {
				splits = append(splits, currentChunk.String())
				currentChunk.Reset()
			}
			subSplits := c.splitTextBySeparators(part, targetSize, nextSeparators)
			splits = append(splits, subSplits...)
			continue
		}

		if currentChunk.Len()+len([]rune(part))+len([]rune(sep)) > targetSize {
			if currentChunk.Len() > 0 {
				splits = append(splits, currentChunk.String())
				currentChunk.Reset()
			}
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString(sep)
		}
		currentChunk.WriteString(part)
	}

	if currentChunk.Len() > 0 {
		splits = append(splits, currentChunk.String())
	}

	return splits
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}
