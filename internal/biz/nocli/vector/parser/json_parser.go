package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// JSONParser 结构化 JSON 树节点节点展平解析策略实现
type JSONParser struct{}

func NewJSONParser() *JSONParser {
	return &JSONParser{}
}

func (p *JSONParser) Parse(ctx context.Context, r io.Reader, filename string) (*ParsedDocument, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var raw interface{}
	if err := json.Unmarshal(buf, &raw); err != nil {
		// 若无法解析为 JSON，直接回退为普通文本
		return &ParsedDocument{
			Title:      filename,
			SourceType: "json",
			Content:    strings.TrimSpace(string(buf)),
		}, nil
	}

	var builder strings.Builder
	p.formatValue(&builder, "", raw)

	content := builder.String()
	secID := "sec_json"
	sec := &DocumentSection{
		SectionID:   secID,
		H1:          filename,
		TitlePath:   filename,
		FullContent: content,
		StartLine:   1,
		LeafNodes: []*DocNode{
			{
				NodeID:    "node_json",
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
		SourceType: "json",
		Content:    content,
		RawContent: content,
		Root:       rootNode,
		Sections:   []*DocumentSection{sec},
		Metadata: map[string]string{
			"parser": "json_flatten_parser",
		},
	}, nil
}

func (p *JSONParser) formatValue(builder *strings.Builder, prefix string, v interface{}) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			newKey := k
			if prefix != "" {
				newKey = prefix + "." + k
			}
			p.formatValue(builder, newKey, child)
		}
	case []interface{}:
		for i, child := range val {
			newKey := fmt.Sprintf("%s[%d]", prefix, i)
			p.formatValue(builder, newKey, child)
		}
	default:
		builder.WriteString(fmt.Sprintf("%s: %v\n", prefix, val))
	}
}
