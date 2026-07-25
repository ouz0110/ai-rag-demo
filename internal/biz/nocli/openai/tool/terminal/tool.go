package terminal

import (
	"ai-rag-demo/internal/conf"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const ToolName = "terminal"

type Args struct {
	Command string `json:"command"`
	Cwd     string `json:"cwd"`
}

type Tool struct {
	cfg *conf.Config
}

func NewTool(cfg *conf.Config) *Tool {
	return &Tool{cfg: cfg}
}

func (t *Tool) RequiresApproval(argsJSON string) bool {
	var args Args
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return true
	}
	return IsDangerousCommand(args.Command)
}

func (t *Tool) Definition() openai.Tool {
	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "要在终端执行的 Shell 命令或脚本调用指令。例如: 'curl -sL https://example.com' 或 'git status'。",
			},
			"cwd": map[string]interface{}{
				"type":        "string",
				"description": "执行命令的工作目录，相对于配置的工作目录。留空表示当前工作目录。",
			},
		},
		"required": []string{"command"},
	}

	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        ToolName,
			Description: "在系统的终端中执行 Shell 命令或脚本调度指令。支持只读探索与工具调用。只读指令自动执行，修改/高危指令需要用户确认。",
			Parameters:  parameters,
		},
	}
}

func (t *Tool) Run(ctx context.Context, argsJSON string) (string, error) {
	var args Args
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 terminal 参数失败: %v", err)
	}

	command := strings.TrimSpace(args.Command)
	if command == "" {
		return "", fmt.Errorf("执行命令不能为空")
	}

	workDir := t.cfg.Source.Nocli.WorkDir
	if workDir == "" {
		workDir = "."
	}

	targetDir := workDir
	if args.Cwd != "" {
		rel := filepath.Clean(args.Cwd)
		if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return "", fmt.Errorf("禁止跨工作目录路径逃逸: %s", args.Cwd)
		}
		targetDir = filepath.Join(workDir, rel)
	}

	// 5 分钟安全超时
	execCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	} else {
		cmd = exec.CommandContext(execCtx, "bash", "-c", command)
	}
	cmd.Dir = targetDir

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	outputStr := strings.TrimSpace(stdoutBuf.String())
	stderrStr := strings.TrimSpace(stderrBuf.String())

	var result strings.Builder
	result.WriteString(fmt.Sprintf("### 🖥️ 终端指令执行结果: `%s`\n\n", command))
	if outputStr != "" {
		result.WriteString("```\n")
		result.WriteString(truncateOutput(outputStr, 4000))
		result.WriteString("\n```\n")
	}
	if stderrStr != "" {
		result.WriteString("**Stderr 输出:**\n```\n")
		result.WriteString(truncateOutput(stderrStr, 2000))
		result.WriteString("\n```\n")
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("终端指令执行超时 (5分钟): %v", err)
		}
		if result.Len() > 0 {
			return result.String(), nil
		}
		return "", fmt.Errorf("终端指令执行失败 [%v]: %s", err, stderrStr)
	}

	if result.Len() == 0 {
		return "指令执行成功（未产生标准输出）", nil
	}

	return result.String(), nil
}

func truncateOutput(str string, maxChars int) string {
	runes := []rune(str)
	if len(runes) > maxChars {
		return string(runes[:maxChars]) + "\n... (输出已截断)"
	}
	return str
}
