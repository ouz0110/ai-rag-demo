package parser

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// CSVParser 二维表格数据自然语言平铺解析策略实现 (Key: Value 属性转换)
type CSVParser struct{}

func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

func (p *CSVParser) Parse(ctx context.Context, r io.Reader, filename string) (*ParsedDocument, error) {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(lines) == 0 {
		return &ParsedDocument{
			Title:      filename,
			SourceType: "csv",
			Content:    "",
		}, nil
	}

	sep := ","
	if strings.HasSuffix(strings.ToLower(filename), ".tsv") {
		sep = "\t"
	}

	headers := strings.Split(lines[0], sep)
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}

	var builder strings.Builder
	for idx, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cols := strings.Split(line, sep)
		builder.WriteString(fmt.Sprintf("记录 %d：", idx+1))
		for cIdx, col := range cols {
			colVal := strings.TrimSpace(col)
			headerName := fmt.Sprintf("列%d", cIdx+1)
			if cIdx < len(headers) && headers[cIdx] != "" {
				headerName = headers[cIdx]
			}
			builder.WriteString(fmt.Sprintf("%s为 %s", headerName, colVal))
			if cIdx < len(cols)-1 {
				builder.WriteString("，")
			}
		}
		builder.WriteString("。\n")
	}

	content := builder.String()
	secID := "sec_csv"
	sec := &DocumentSection{
		SectionID:   secID,
		H1:          filename,
		TitlePath:   filename,
		FullContent: content,
		StartLine:   1,
		LeafNodes: []*DocNode{
			{
				NodeID:    "node_csv",
				ParentID:  secID,
				Type:      NodeTable,
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
		SourceType: "csv",
		Content:    content,
		RawContent: content,
		Root:       rootNode,
		Sections:   []*DocumentSection{sec},
		Metadata: map[string]string{
			"parser": "csv_table_flatten_parser",
		},
	}, nil
}
