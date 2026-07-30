package list_file

import (
	"ai-rag-demo/internal/biz/nocli/openai/tool/toolutil"
	"ai-rag-demo/internal/conf"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const ToolName = "list_files"

type Args struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

type Tool struct {
	cfg *conf.Config
}

func NewTool(cfg *conf.Config) *Tool {
	toolutil.ParseFilters(cfg)
	return &Tool{cfg: cfg}
}

func (t *Tool) RequiresApproval(ctx context.Context, argsJSON string) bool {
	return false
}

func (t *Tool) Definition() openai.Tool {
	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "要列出的目录路径，相对于工作目录。留空或使用 '.' 表示当前工作目录。",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "是否递归列出子目录，最多递归5层。默认 true。",
			},
		},
		"required": []string{"path"},
	}

	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        ToolName,
			Description: "列出指定目录下的文件列表。支持递归，最多5层。所有路径都相对于配置的工作目录。",
			Parameters:  parameters,
		},
	}
}

func (t *Tool) Run(ctx context.Context, argsJSON string) (string, error) {
	var args Args
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("参数解析失败: %v", err)
	}

	path := args.Path
	if path == "" {
		path = "."
	}
	recursive := args.Recursive

	res, err := toolutil.ResolvePathWithCtx(ctx, t.cfg, path)
	if err != nil {
		return "", err
	}

	maxDepth := 5
	if !recursive {
		maxDepth = 0
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("### 📁 目录列表：%s\n\n", res.RelPath))
	result.WriteString("| 类型 | 路径 | 大小 |\n")
	result.WriteString("|------|------|------|\n")

	err = t.listFiles(res.Target, res.WorkDir, maxDepth, 0, &result)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

func (t *Tool) listFiles(dir, workDir string, maxDepth, currentDepth int, result *strings.Builder) error {
	if maxDepth >= 0 && currentDepth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("读取目录失败 %s: %v", dir, err)
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		relPath, err := filepath.Rel(workDir, fullPath)
		if err != nil {
			continue
		}

		base := filepath.Base(relPath)
		if toolutil.ShouldIgnore(base) {
			continue
		}

		if entry.IsDir() {
			info, err := entry.Info()
			size := "-"
			if err == nil {
				size = fmt.Sprintf("%d", info.Size())
			}
			fmt.Fprintf(result, "| 📂 目录 | `%s/` | %s |\n", relPath, size)
			if maxDepth < 0 || currentDepth < maxDepth {
				if err := t.listFiles(fullPath, workDir, maxDepth, currentDepth+1, result); err != nil {
					return err
				}
			}
		} else {
			if !toolutil.IsAllowedFile(entry.Name()) {
				continue
			}
			info, err := entry.Info()
			size := "-"
			if err == nil {
				size = fmt.Sprintf("%d", info.Size())
			}
			fmt.Fprintf(result, "| 📄 文件 | `%s` | %s |\n", relPath, size)
		}
	}

	return nil
}
