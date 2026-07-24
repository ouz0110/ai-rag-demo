package readfiles

import (
	"ai-rag-demo/internal/conf"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	openai "github.com/sashabaranov/go-openai"

	list_file "ai-rag-demo/internal/biz/nocli/openai/tool/list_files"
)

const ToolName = "read_files"

type Args struct {
	Files     []string `json:"files"`
	StartLine int      `json:"start_line"`
	EndLine   int      `json:"end_line"`
	MaxFiles  int      `json:"max_files"`
	MaxBytes  int      `json:"max_bytes"`
}

type Tool struct {
	cfg             *conf.Config
	maxReadFiles    int
	maxTotalBytes   int
	chunkLines      int
}

func NewTool(cfg *conf.Config) *Tool {
	list_file.ParseFilters(cfg)

	maxReadFiles := cfg.Source.Nocli.MaxReadFiles
	if maxReadFiles <= 0 {
		maxReadFiles = 10
	}

	maxTotalBytes := cfg.Source.Nocli.MaxTotalBytes
	if maxTotalBytes <= 0 {
		maxTotalBytes = 50 * 1024
	}

	chunkLines := cfg.Source.Nocli.ChunkLines
	if chunkLines <= 0 {
		chunkLines = 100
	}

	return &Tool{
		cfg:             cfg,
		maxReadFiles:    maxReadFiles,
		maxTotalBytes:   maxTotalBytes,
		chunkLines:      chunkLines,
	}
}

func (t *Tool) Definition() openai.Tool {
	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"files": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "要读取的文件路径列表，相对于工作目录。禁止使用绝对路径或 .. 逃逸，系统会自动校验。",
			},
			"start_line": map[string]interface{}{
				"type":        "integer",
				"description": "起始行号，从1开始。默认1。",
			},
			"end_line": map[string]interface{}{
				"type":        "integer",
				"description": "结束行号，0表示读取到文件末尾或块末尾。默认0。",
			},
			"max_files": map[string]interface{}{
				"type":        "integer",
				"description": "单次最大读取文件数，超过则截断。默认使用配置值。",
			},
			"max_bytes": map[string]interface{}{
				"type":        "integer",
				"description": "单次最大读取字节数，超过则截断。默认使用配置值。",
			},
		},
		"required": []string{"files"},
	}

	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        ToolName,
			Description: "批量读取指定文件内容。支持行范围限制和总字节数限制。大文件会自动分段读取。结果供模型分析使用，不直接返回给用户。",
			Parameters: parameters,
		},
	}
}

func (t *Tool) Run(ctx context.Context, argsJSON string) (string, error) {
	var args Args
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("参数解析失败: %v", err)
	}

	rawFiles := args.Files
	if len(rawFiles) == 0 {
		return "", fmt.Errorf("files 参数不能为空")
	}

	startLine := args.StartLine
	if startLine < 1 {
		startLine = 1
	}
	endLine := args.EndLine

	maxFiles := args.MaxFiles
	if maxFiles <= 0 {
		maxFiles = t.maxReadFiles
	}

	maxBytes := args.MaxBytes
	if maxBytes <= 0 {
		maxBytes = t.maxTotalBytes
	}

	chunkLines := t.chunkLines

	var result strings.Builder
	totalBytes := 0
	fileCount := 0

	for _, path := range rawFiles {
		if fileCount >= maxFiles {
			result.WriteString(fmt.Sprintf("[TRUNCATED: 超过最大读取文件数限制 %d]\n", maxFiles))
			break
		}

		res, err := list_file.ResolvePath(t.cfg, path)
		if err != nil {
			result.WriteString(fmt.Sprintf("--- FILE: %s ---\nERROR: %s\n--- END FILE ---\n\n", path, err))
			continue
		}

		content, linesRead, totalLines, err := readFileChunk(res.Target, startLine, endLine, chunkLines)
		if err != nil {
			result.WriteString(fmt.Sprintf("--- FILE: %s ---\nERROR: %s\n--- END FILE ---\n\n", res.RelPath, err))
			continue
		}

		if totalBytes+RuneLen(content) > maxBytes {
			allowed := maxBytes - totalBytes
			if allowed > 0 {
				truncated := truncateToRuneBoundary(content, allowed)
				result.WriteString(fmt.Sprintf("--- FILE: %s (lines %d-%d, %d bytes) ---\n", res.RelPath, startLine, startLine+linesRead-1, RuneLen(truncated)))
				result.WriteString(Desensitize(truncated))
				result.WriteString("\n--- END FILE ---\n\n")
				totalBytes += RuneLen(truncated)
			}
			result.WriteString(fmt.Sprintf("[TRUNCATED: 超过最大读取字符数限制 %d]\n", maxBytes))
			break
		}

		note := ""
		if endLine == 0 && totalLines > startLine+linesRead-1 {
			note = fmt.Sprintf(" [NOTE: 文件共 %d 行，当前读取 %d-%d，可继续读取]",
				totalLines, startLine, startLine+linesRead-1)
		}

		result.WriteString(fmt.Sprintf("--- FILE: %s (lines %d-%d, %d chars/%d bytes)%s ---\n",
			res.RelPath, startLine, startLine+linesRead-1, RuneLen(content), len(content), note))
		result.WriteString(Desensitize(content))
		result.WriteString("\n--- END FILE ---\n\n")

		totalBytes += RuneLen(content)
		fileCount++
	}

	return result.String(), nil
}

func readFileChunk(path string, startLine, endLine, chunkLines int) (string, int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, 0, err
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	if startLine < 1 {
		startLine = 1
	}
	if startLine > totalLines {
		return "", 0, totalLines, nil
	}

	actualEnd := endLine
	if actualEnd == 0 || actualEnd > totalLines {
		actualEnd = totalLines
	}
	if endLine == 0 {
		actualEnd = startLine + chunkLines - 1
	}
	if actualEnd > totalLines {
		actualEnd = totalLines
	}
	if actualEnd < startLine {
		actualEnd = startLine
	}

	chunk := strings.Join(lines[startLine-1:actualEnd], "\n")
	return chunk, actualEnd - startLine + 1, totalLines, nil
}

func truncateToRuneBoundary(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	for i := maxBytes; i > 0; i-- {
		if utf8.ValidString(s[:i]) {
			return s[:i]
		}
	}
	return ""
}
