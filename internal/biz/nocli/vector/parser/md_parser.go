package parser

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// MarkdownParser 高性能 Markdown 层级结构化 AST 解析器
type MarkdownParser struct{}

func NewMarkdownParser() *MarkdownParser {
	return &MarkdownParser{}
}

func (p *MarkdownParser) Parse(ctx context.Context, r io.Reader, filename string) (*ParsedDocument, error) {
	scanner := bufio.NewScanner(r)
	// 适当放宽 Scanner 单行最大 Buffer，防止超长代码块或表格行溢出 (10MB)
	const maxCapacity = 10 * 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read markdown failed: %w", err)
	}

	rawContent := strings.Join(lines, "\n")
	docTitle := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	// 尝试将 lines 解析为叶子语法节点 (Leaf Nodes)
	rawNodes := parseMarkdownLinesToNodes(lines)

	// 构建全局 AST 根节点与层次结构树
	rootNode, sections := buildTreeAndSections(docTitle, rawNodes)

	return &ParsedDocument{
		DocID:      uuid.New().String(),
		Title:      docTitle,
		SourceType: "md",
		Content:    rawContent,
		RawContent: rawContent,
		Root:       rootNode,
		Sections:   sections,
		GlobalMeta: map[string]string{
			"total_lines": fmt.Sprintf("%d", len(lines)),
		},
		Metadata: map[string]string{
			"title": docTitle,
		},
	}, nil
}

// parseMarkdownLinesToNodes 按照 Markdown 行状态机，将原始行数组解析为粗粒度叶子语法节点数组
func parseMarkdownLinesToNodes(lines []string) []*DocNode {
	var nodes []*DocNode

	inCodeBlock := false
	var codeBlockLines []string
	codeBlockStartLine := 0

	inTable := false
	var tableLines []string
	tableStartLine := 0

	inParagraph := false
	var paragraphLines []string
	paragraphStartLine := 0

	flushParagraph := func(endLine int) {
		if len(paragraphLines) > 0 {
			nodes = append(nodes, &DocNode{
				NodeID:    uuid.New().String(),
				Type:      NodeParagraph,
				Content:   strings.TrimSpace(strings.Join(paragraphLines, "\n")),
				StartLine: paragraphStartLine,
				EndLine:   endLine,
			})
			paragraphLines = nil
			inParagraph = false
		}
	}

	flushTable := func(endLine int) {
		if len(tableLines) > 0 {
			nodes = append(nodes, &DocNode{
				NodeID:    uuid.New().String(),
				Type:      NodeTable,
				Content:   strings.TrimSpace(strings.Join(tableLines, "\n")),
				StartLine: tableStartLine,
				EndLine:   endLine,
				Metadata:  map[string]string{"has_table": "true"},
			})
			tableLines = nil
			inTable = false
		}
	}

	for i, line := range lines {
		lineNum := i + 1 // 1-indexed
		trimmed := strings.TrimSpace(line)

		// 1. 代码块 ` ``` ` 状态机切换
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				// 代码块闭合
				codeBlockLines = append(codeBlockLines, line)
				nodes = append(nodes, &DocNode{
					NodeID:    uuid.New().String(),
					Type:      NodeCodeBlock,
					Content:   strings.Join(codeBlockLines, "\n"),
					StartLine: codeBlockStartLine,
					EndLine:   lineNum,
					Metadata:  map[string]string{"has_code": "true"},
				})
				codeBlockLines = nil
				inCodeBlock = false
			} else {
				// 冲刷之前可能存在的段落或表格
				flushParagraph(lineNum - 1)
				flushTable(lineNum - 1)

				// 开启代码块
				inCodeBlock = true
				codeBlockStartLine = lineNum
				codeBlockLines = []string{line}
			}
			continue
		}

		if inCodeBlock {
			codeBlockLines = append(codeBlockLines, line)
			continue
		}

		// 2. 表格 `|` 识别状态机
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			if !inTable {
				flushParagraph(lineNum - 1)
				inTable = true
				tableStartLine = lineNum
				tableLines = nil
			}
			tableLines = append(tableLines, line)
			continue
		} else if inTable {
			// 表格结束
			flushTable(lineNum - 1)
		}

		// 3. 标题 Heading 识别 (`# ` ~ `###### `)
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for _, ch := range trimmed {
				if ch == '#' {
					level++
				} else {
					break
				}
			}
			if level >= 1 && level <= 6 && len(trimmed) > level && trimmed[level] == ' ' {
				flushParagraph(lineNum - 1)

				titleText := strings.TrimSpace(trimmed[level+1:])
				nodes = append(nodes, &DocNode{
					NodeID:       uuid.New().String(),
					Type:         NodeHeading,
					HeadingLevel: level,
					Title:        titleText,
					Content:      trimmed,
					StartLine:    lineNum,
					EndLine:      lineNum,
				})
				continue
			}
		}

		// 4. 空行划分段落
		if trimmed == "" {
			flushParagraph(lineNum - 1)
			continue
		}

		// 5. 普通段落行累加
		if !inParagraph {
			inParagraph = true
			paragraphStartLine = lineNum
			paragraphLines = nil
		}
		paragraphLines = append(paragraphLines, line)
	}

	// 最终收尾冲刷
	flushParagraph(len(lines))
	flushTable(len(lines))
	if inCodeBlock && len(codeBlockLines) > 0 {
		nodes = append(nodes, &DocNode{
			NodeID:    uuid.New().String(),
			Type:      NodeCodeBlock,
			Content:   strings.Join(codeBlockLines, "\n"),
			StartLine: codeBlockStartLine,
			EndLine:   len(lines),
			Metadata:  map[string]string{"has_code": "true"},
		})
	}

	return nodes
}

// buildTreeAndSections 根据 AST 节点的 Heading 层级构造多叉语法树与二级 Section 容器列表
func buildTreeAndSections(docTitle string, nodes []*DocNode) (*DocNode, []*DocumentSection) {
	root := &DocNode{
		NodeID:    uuid.New().String(),
		Type:      NodeRoot,
		Title:     docTitle,
		StartLine: 1,
		Children:  nodes,
	}

	if len(nodes) == 0 {
		return root, nil
	}

	var sections []*DocumentSection
	currentH1 := docTitle
	currentH2 := ""
	currentH3 := ""

	var currentSectionNodes []*DocNode
	secStartLine := 1

	flushSection := func(endLine int) {
		if len(currentSectionNodes) == 0 {
			return
		}

		var breadcrumbs []string
		if currentH1 != "" {
			breadcrumbs = append(breadcrumbs, currentH1)
		}
		if currentH2 != "" {
			breadcrumbs = append(breadcrumbs, currentH2)
		}
		if currentH3 != "" {
			breadcrumbs = append(breadcrumbs, currentH3)
		}
		titlePath := strings.Join(breadcrumbs, " > ")

		var contentParts []string
		for _, n := range currentSectionNodes {
			contentParts = append(contentParts, n.Content)
		}

		secID := uuid.New().String()
		section := &DocumentSection{
			SectionID:   secID,
			H1:          currentH1,
			H2:          currentH2,
			H3:          currentH3,
			TitlePath:   titlePath,
			FullContent: strings.Join(contentParts, "\n\n"),
			StartLine:   secStartLine,
			EndLine:     endLine,
			LeafNodes:   currentSectionNodes,
		}

		// 为节点打上面包屑
		for _, n := range currentSectionNodes {
			n.ParentID = secID
			n.Breadcrumbs = breadcrumbs
		}

		sections = append(sections, section)
		currentSectionNodes = nil
	}

	for _, node := range nodes {
		if node.Type == NodeHeading {
			// 每遇到 H1(1) 或 H2(2)，触发产生新的逻辑 Section 父块
			if node.HeadingLevel <= 2 {
				flushSection(node.StartLine - 1)
				secStartLine = node.StartLine
			}

			switch node.HeadingLevel {
			case 1:
				currentH1 = node.Title
				currentH2 = ""
				currentH3 = ""
			case 2:
				currentH2 = node.Title
				currentH3 = ""
			case 3:
				currentH3 = node.Title
			}
		}

		currentSectionNodes = append(currentSectionNodes, node)
	}

	flushSection(nodes[len(nodes)-1].EndLine)

	return root, sections
}
