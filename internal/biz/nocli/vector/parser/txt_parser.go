package parser

import (
	"context"
	"io"

	"strings"
)

// TextParser 纯文本与 Markdown 文档解析策略实现
type TextParser struct{}

func NewTextParser() *TextParser {
	return &TextParser{}
}

func (p *TextParser) Parse(ctx context.Context, r io.Reader, filename string) (*ParsedDocument, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(string(buf))
	ext := "txt"
	if idx := strings.LastIndex(filename, "."); idx != -1 {
		ext = strings.ToLower(filename[idx+1:])
	}

	secID := "sec_default"
	sec := &DocumentSection{
		SectionID:   secID,
		H1:          filename,
		TitlePath:   filename,
		FullContent: content,
		StartLine:   1,
		LeafNodes: []*DocNode{
			{
				NodeID:    "node_default",
				ParentID:  secID,
				Type:      NodeParagraph,
				Content:   content,
				StartLine: 1,
			},
		},
	}

	rootNode := &DocNode{
		NodeID:    "root",
		Type:      NodeRoot,
		Title:     filename,
		StartLine: 1,
		Children:  sec.LeafNodes,
	}

	return &ParsedDocument{
		DocID:      secID,
		Title:      filename,
		SourceType: ext,
		Content:    content,
		RawContent: content,
		Root:       rootNode,
		Sections:   []*DocumentSection{sec},
		Metadata: map[string]string{
			"parser": "text_parser",
		},
	}, nil
}
