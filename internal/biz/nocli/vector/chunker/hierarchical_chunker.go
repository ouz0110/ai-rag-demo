package chunker

import (
	"strings"

	"ai-rag-demo/internal/biz/nocli/vector/parser"

	"github.com/google/uuid"
)

// HierarchicalChunker 通用语法树驱动的层级切片算子 (Hierarchical AST Chunker)
type HierarchicalChunker struct {
	maxChildSize int // 子块合适字符长度，默认 800
	minChildSize int // 碎片合并门槛，默认 50
}

func NewHierarchicalChunker(maxSize, minSize int) *HierarchicalChunker {
	if maxSize <= 0 {
		maxSize = 800
	}
	if minSize <= 0 {
		minSize = 50
	}
	return &HierarchicalChunker{
		maxChildSize: maxSize,
		minChildSize: minSize,
	}
}

// SplitFromAST 核心入口：直接根据通用 ParsedDocument 语法树进行三层架构切片
func (hc *HierarchicalChunker) SplitFromAST(doc *parser.ParsedDocument) *ParentChildResult {
	if doc == nil || len(doc.Sections) == 0 {
		return &ParentChildResult{}
	}

	var parentChunks []*ChunkUnit
	var childChunks []*ChunkUnit
	globalChildIdx := int32(0)

	for pIdx, sec := range doc.Sections {
		secParentID := sec.SectionID
		if secParentID == "" {
			secParentID = fmtUUID()
		}

		// 1. 构造二级块：章节父块 (Parent Section Chunk)
		parentUnit := &ChunkUnit{
			ChunkID:    secParentID,
			ParentID:   "",
			H1:         sec.H1,
			H2:         sec.H2,
			H3:         sec.H3,
			StartLine:  int32(sec.StartLine),
			EndLine:    int32(sec.EndLine),
			ChunkType:  "parent",
			Content:    sec.FullContent,
			ChunkIndex: int32(pIdx),
			TokenCount: int32(len([]rune(sec.FullContent))),
			IsParent:   true,
		}
		parentChunks = append(parentChunks, parentUnit)

		// 2. 遍历该章节下的 Leaf 语法节点构造三级块 (Child Leaf Chunks)
		var pendingChildText strings.Builder
		var pendingStartLine, pendingEndLine int
		var pendingHasTable, pendingHasCode int32

		flushPendingChild := func() {
			text := strings.TrimSpace(pendingChildText.String())
			if isIgnoredNoiseNode(text) {
				pendingChildText.Reset()
				return
			}

			childUnit := &ChunkUnit{
				ChunkID:    fmtUUID(),
				ParentID:   secParentID,
				H1:         sec.H1,
				H2:         sec.H2,
				H3:         sec.H3,
				StartLine:  int32(pendingStartLine),
				EndLine:    int32(pendingEndLine),
				HasTable:   pendingHasTable,
				HasCode:    pendingHasCode,
				ChunkType:  "text",
				Content:    text,
				ChunkIndex: globalChildIdx,
				TokenCount: int32(len([]rune(text))),
				IsParent:   false,
			}
			if pendingHasTable == 1 {
				childUnit.ChunkType = "table"
			} else if pendingHasCode == 1 {
				childUnit.ChunkType = "code"
			}

			childChunks = append(childChunks, childUnit)
			globalChildIdx++
			pendingChildText.Reset()
			pendingHasTable = 0
			pendingHasCode = 0
		}

		for _, node := range sec.LeafNodes {
			// 保护逻辑 1：表格或代码块不可切分单元
			if node.Type == parser.NodeTable || node.Type == parser.NodeCodeBlock {
				// 先冲刷之前积攒的普通文本段落
				flushPendingChild()

				hasTab := int32(0)
				hasCd := int32(0)
				cType := "text"
				if node.Type == parser.NodeTable {
					hasTab = 1
					cType = "table"
				} else {
					hasCd = 1
					cType = "code"
				}

				nodeText := strings.TrimSpace(node.Content)
				if isIgnoredNoiseNode(nodeText) {
					continue
				}

				childUnit := &ChunkUnit{
					ChunkID:    fmtUUID(),
					ParentID:   secParentID,
					H1:         sec.H1,
					H2:         sec.H2,
					H3:         sec.H3,
					StartLine:  int32(node.StartLine),
					EndLine:    int32(node.EndLine),
					HasTable:   hasTab,
					HasCode:    hasCd,
					ChunkType:  cType,
					Content:    nodeText,
					ChunkIndex: globalChildIdx,
					TokenCount: int32(len([]rune(nodeText))),
					IsParent:   false,
				}
				childChunks = append(childChunks, childUnit)
				globalChildIdx++
				continue
			}

			// 保护逻辑 2：普通文本段落的累加与分块
			nodeText := strings.TrimSpace(node.Content)
			if isIgnoredNoiseNode(nodeText) {
				continue
			}

			if pendingChildText.Len() == 0 {
				pendingStartLine = node.StartLine
			}
			pendingEndLine = node.EndLine

			if pendingChildText.Len()+len(nodeText) <= hc.maxChildSize {
				if pendingChildText.Len() > 0 {
					pendingChildText.WriteString("\n\n")
				}
				pendingChildText.WriteString(nodeText)
			} else {
				// 已达到最大尺寸，先冲刷
				flushPendingChild()
				pendingStartLine = node.StartLine
				pendingEndLine = node.EndLine

				// 若单个段落本身就超长 (> maxChildSize)，在自然标点符号处切分
				if len([]rune(nodeText)) > hc.maxChildSize {
					subPieces := splitLongTextByPunctuation(nodeText, hc.maxChildSize)
					for _, piece := range subPieces {
						if isIgnoredNoiseNode(piece) {
							continue
						}
						childChunks = append(childChunks, &ChunkUnit{
							ChunkID:    fmtUUID(),
							ParentID:   secParentID,
							H1:         sec.H1,
							H2:         sec.H2,
							H3:         sec.H3,
							StartLine:  int32(node.StartLine),
							EndLine:    int32(node.EndLine),
							ChunkType:  "text",
							Content:    piece,
							ChunkIndex: globalChildIdx,
							TokenCount: int32(len([]rune(piece))),
							IsParent:   false,
						})
						globalChildIdx++
					}
				} else {
					pendingChildText.WriteString(nodeText)
				}
			}
		}

		flushPendingChild()
	}

	return &ParentChildResult{
		ParentChunks: parentChunks,
		ChildChunks:  childChunks,
	}
}

func fmtUUID() string {
	return uuid.New().String()
}

// splitLongTextByPunctuation 在句号/问号/叹号等中文/英文自然标点边界对超长句子进行安全平滑切分
func splitLongTextByPunctuation(text string, maxRuneLen int) []string {
	runes := []rune(text)
	if len(runes) <= maxRuneLen {
		return []string{text}
	}

	var pieces []string
	start := 0
	for start < len(runes) {
		end := start + maxRuneLen
		if end >= len(runes) {
			pieces = append(pieces, string(runes[start:]))
			break
		}

		// 向后寻找距离 end 最近的标点符号位置
		cutPos := end
		for i := end; i > start+maxRuneLen/2; i-- {
			ch := runes[i]
			if ch == '。' || ch == '？' || ch == '！' || ch == '.' || ch == '?' || ch == '!' || ch == '\n' {
				cutPos = i + 1
				break
			}
		}

		pieces = append(pieces, strings.TrimSpace(string(runes[start:cutPos])))
		start = cutPos
	}

	return pieces
}

func isIgnoredNoiseNode(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" || t == "***" || t == "---" || t == "___" || t == "***\n" {
		return true
	}
	return false
}

