package ragtool

import (
	"testing"
)

func TestRewriteToStandardQueries(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "价格类模糊提问规整",
			input:    "请问帮我查一下手机那个价格",
			contains: "手机 最新价格与历史售价变动说明",
		},
		{
			name:     "参数限制模糊提问规整",
			input:    "帮我查下系统配置限制",
			contains: "系统 详细配置参数与功能规格限制说明",
		},
		{
			name:     "安装教程模糊提问规整",
			input:    "那个软件怎么安装",
			contains: "软件 安装部署步骤与操作指南教程",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := RewriteToStandardQueries(tt.input)
			if len(queries) == 0 {
				t.Fatalf("RewriteToStandardQueries returned empty list")
			}
			found := false
			for _, q := range queries {
				if q == tt.contains {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected rewritten query to contain '%s', got: %v", tt.contains, queries)
			}
		})
	}
}
