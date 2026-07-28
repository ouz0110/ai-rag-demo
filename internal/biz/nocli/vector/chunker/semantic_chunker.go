package chunker

import (
	"context"
	"fmt"
	"math"
	"strings"

	"ai-rag-demo/internal/biz/nocli/vector/embedder"
	"ai-rag-demo/internal/conf"

	"github.com/google/uuid"
	openai "github.com/sashabaranov/go-openai"
)

// SemanticChunker 两阶段语义感知动态切片算子
type SemanticChunker struct {
	MaxChunkSize   int                // 单个 Parent Chunk 的硬上限限制 (如 1200 字符)
	BaseChunkSize  int                // 底包原子 Block 期望长度 (如 512 字符)
	SplitThreshold float64            // 语义断层切割阈值 (余弦相似度低于该值执行话题切割，如 0.65)
	embedder       *embedder.Embedder // 向量 Embedding 计算器
}

// NewSemanticChunkerFromConfig 从 Config 配置句柄创建 SemanticChunker
func NewSemanticChunkerFromConfig(cfg *conf.Config, emb *embedder.Embedder) *SemanticChunker {
	maxSize := 1200
	baseSize := 512
	sThreshold := 0.65

	if cfg != nil && cfg.Source.RAG != nil && cfg.Source.RAG.Chunker != nil {
		cCfg := cfg.Source.RAG.Chunker
		if cCfg.ParentSize > 0 {
			maxSize = cCfg.ParentSize
		}
		if cCfg.ChildSize > 0 {
			baseSize = cCfg.ChildSize
		}
		if cCfg.SplitThreshold > 0 {
			sThreshold = float64(cCfg.SplitThreshold)
		}
	}

	return &SemanticChunker{
		MaxChunkSize:   maxSize,
		BaseChunkSize:  baseSize,
		SplitThreshold: sThreshold,
		embedder:       emb,
	}
}

// SplitWithSemantics 执行两阶段语义感知动态切片算法，并返回过程中的 Token Usage 消耗
func (c *SemanticChunker) SplitWithSemantics(ctx context.Context, text string) (*ParentChildResult, openai.Usage, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return &ParentChildResult{}, openai.Usage{}, nil
	}

	// 阶段 1：使用递归字符切分算法将长文档切成 512 字符左右的原子“底包”
	baseChunks := c.recursiveSplit(text, c.BaseChunkSize)
	if len(baseChunks) == 0 {
		return &ParentChildResult{}, openai.Usage{}, nil
	}
	if len(baseChunks) == 1 {
		pID := fmt.Sprintf("p_%s", uuid.New().String())
		cID := fmt.Sprintf("c_%s", uuid.New().String())
		unitP := &ChunkUnit{ChunkID: pID, Content: baseChunks[0], TokenCount: int32(len([]rune(baseChunks[0]))), IsParent: true}
		unitC := &ChunkUnit{ChunkID: cID, ParentID: pID, Content: baseChunks[0], TokenCount: int32(len([]rune(baseChunks[0]))), IsParent: false}
		return &ParentChildResult{
			ParentChunks: []*ChunkUnit{unitP},
			ChildChunks:  []*ChunkUnit{unitC},
		}, openai.Usage{}, nil
	}

	// 阶段 2：批量计算所有底包的 Embedding 向量
	vectors, semUsage, err := c.embedder.BatchGenerateEmbeddings(ctx, baseChunks)
	if err != nil || len(vectors) != len(baseChunks) {
		// 若向量生成失败，降级回退到静态规则切分
		staticChunker := NewStaticChunker(c.MaxChunkSize, c.BaseChunkSize, 30)
		return staticChunker.Split(text), semUsage, nil
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

		// 判断是否需要切割 Parent Chunk
		shouldSplit := false
		if i < len(baseChunks)-1 {
			sim := cosineSimilarity(vectors[i], vectors[i+1])
			if sim < float32(c.SplitThreshold) {
				shouldSplit = true // 余弦相似度低于阈值，判断为话题断层，执行切割
			}
		} else {
			shouldSplit = true // 最后一个 chunk 自动闭合当前 Parent
		}

		// 容忍度校验：若当前 Parent 内容已接近/超过 MaxChunkSize，强制切割
		if currentParentBuilder.Len() >= c.MaxChunkSize {
			shouldSplit = true
		}

		if shouldSplit {
			pContent := currentParentBuilder.String()
			parentUnit := &ChunkUnit{
				ChunkID:    currentParentID,
				Content:    pContent,
				ChunkIndex: int32(parentIndex),
				TokenCount: int32(len([]rune(pContent))),
				IsParent:   true,
			}
			parentUnits = append(parentUnits, parentUnit)
			parentIndex++

			// 重置 Builder 与 ID 准备开启下一个 Parent Chunk
			currentParentBuilder.Reset()
			currentParentID = fmt.Sprintf("p_%s", uuid.New().String())
		}
	}

	return &ParentChildResult{
		ParentChunks: parentUnits,
		ChildChunks:  childUnits,
	}, semUsage, nil
}

// recursiveSplit 递归字符切分算法实现
func (c *SemanticChunker) recursiveSplit(text string, targetSize int) []string {
	separators := []string{"\n\n", "\n", "。", "！", "？", ".", "!", "?", " ", ""}
	return c.splitText(text, targetSize, separators)
}

func (c *SemanticChunker) splitText(text string, targetSize int, separators []string) []string {
	if len([]rune(text)) <= targetSize || len(separators) == 0 {
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []string{text}
	}

	sep := separators[0]
	nextSeparators := separators[1:]
	var parts []string

	if sep == "" {
		runes := []rune(text)
		for i := 0; i < len(runes); i += targetSize {
			end := i + targetSize
			if end > len(runes) {
				end = len(runes)
			}
			parts = append(parts, string(runes[i:end]))
		}
	} else {
		parts = strings.Split(text, sep)
	}

	var result []string
	var currentBuilder strings.Builder

	for _, part := range parts {
		if part == "" {
			continue
		}

		partLen := len([]rune(part))
		if partLen > targetSize {
			if currentBuilder.Len() > 0 {
				result = append(result, currentBuilder.String())
				currentBuilder.Reset()
			}
			subParts := c.splitText(part, targetSize, nextSeparators)
			result = append(result, subParts...)
			continue
		}

		if currentBuilder.Len()+partLen+1 > targetSize {
			result = append(result, currentBuilder.String())
			currentBuilder.Reset()
		}

		if currentBuilder.Len() > 0 {
			currentBuilder.WriteString(sep)
		}
		currentBuilder.WriteString(part)
	}

	if currentBuilder.Len() > 0 {
		result = append(result, currentBuilder.String())
	}

	return result
}

// cosineSimilarity 计算两个 32 位浮点向量的余弦相似度
func cosineSimilarity(v1, v2 []float32) float32 {
	if len(v1) == 0 || len(v2) == 0 || len(v1) != len(v2) {
		return 0
	}

	var dotProduct float64
	var normA float64
	var normB float64

	for i := 0; i < len(v1); i++ {
		a := float64(v1[i])
		b := float64(v2[i])
		dotProduct += a * b
		normA += a * a
		normB += b * b
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}
