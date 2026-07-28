package chunker

import (
	"context"
	"strings"
	"testing"

	"ai-rag-demo/internal/biz/nocli/vector/parser"
)

func TestHierarchicalChunker(t *testing.T) {
	mdContent := `# API 规范文档

这是背景简介。

## 一、接口命名

所有 API 接口采用小驼峰命名。

| 接口 | 方法 | 说明 |
| --- | --- | --- |
| /user/get | GET | 获取用户 |
| /user/add | POST | 添加用户 |

## 二、响应格式

格式如下：

` + "```json" + `
{
  "code": 0,
  "msg": "ok"
}
` + "```" + `
`

	p := parser.NewMarkdownParser()
	doc, err := p.Parse(context.Background(), strings.NewReader(mdContent), "api_spec.md")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	chk := NewHierarchicalChunker(800, 50)
	res := chk.SplitFromAST(doc)

	if len(res.ParentChunks) != 3 {
		t.Fatalf("expected 3 parent chunks, got %d", len(res.ParentChunks))
	}

	if len(res.ChildChunks) == 0 {
		t.Fatalf("expected child chunks, got 0")
	}

	foundTable := false
	foundCode := false
	for _, c := range res.ChildChunks {
		if c.HasTable == 1 && c.ChunkType == "table" {
			foundTable = true
			if !strings.Contains(c.Content, "API 规范文档 > 一、接口命名") {
				t.Errorf("expected metadata prefix in table child chunk, got: %s", c.Content)
			}
		}
		if c.HasCode == 1 && c.ChunkType == "code" {
			foundCode = true
			if !strings.Contains(c.Content, "API 规范文档 > 二、响应格式") {
				t.Errorf("expected metadata prefix in code child chunk, got: %s", c.Content)
			}
		}
	}

	if !foundTable {
		t.Errorf("expected table child chunk with HasTable=1")
	}
	if !foundCode {
		t.Errorf("expected code child chunk with HasCode=1")
	}
}
