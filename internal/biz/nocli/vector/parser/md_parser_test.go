package parser

import (
	"context"
	"strings"
	"testing"
)

func TestMarkdownParser(t *testing.T) {
	mdContent := `# 快速指南

这是一段前言段落。

## 一、系统配置

配置系统的说明文本。

| 配置项 | 类型 | 默认值 |
| --- | --- | --- |
| Port | int | 8080 |
| Debug | bool | true |

## 二、代码示例

以下是 Go 语言代码：

` + "```go" + `
func main() {
    println("Hello World")
}
` + "```" + `

段落结束。
`

	parser := NewMarkdownParser()
	doc, err := parser.Parse(context.Background(), strings.NewReader(mdContent), "demo.md")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if doc.Title != "demo" {
		t.Errorf("expected title demo, got %s", doc.Title)
	}

	if len(doc.Sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(doc.Sections))
	}

	sec1 := doc.Sections[0]
	if sec1.H1 != "快速指南" {
		t.Errorf("expected section 1 H1 '快速指南', got '%s'", sec1.H1)
	}

	sec2 := doc.Sections[1]
	if sec2.H2 != "一、系统配置" {
		t.Errorf("expected section 2 H2 '一、系统配置', got '%s'", sec2.H2)
	}

	hasTable := false
	for _, node := range sec2.LeafNodes {
		if node.Type == NodeTable {
			hasTable = true
			break
		}
	}
	if !hasTable {
		t.Errorf("expected table node in section 2")
	}

	sec3 := doc.Sections[2]
	hasCode := false
	for _, node := range sec3.LeafNodes {
		if node.Type == NodeCodeBlock {
			hasCode = true
			break
		}
	}
	if !hasCode {
		t.Errorf("expected code node in section 3")
	}
}
