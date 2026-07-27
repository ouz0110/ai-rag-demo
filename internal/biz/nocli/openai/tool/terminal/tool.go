package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ai-rag-demo/internal/conf"

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

	workDir := t.cfg.Source.Nocli.WorkDir
	if workDir == "" {
		workDir = "."
	}

	// 1. 任何企图逃逸工作目录的 cwd 或 command 均属于风险/越界操作，强制触发人工审批
	if HasPathEscape(workDir, args.Cwd) || HasPathEscape(workDir, args.Command) {
		return true
	}

	return IsDangerousCommand(workDir, args.Command)
}

func (t *Tool) Definition() openai.Tool {
	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "要在终端执行的 Shell 命令或脚本调用指令。注意：所有命令及其操作的目标路径必须限制在当前工作目录内部，严禁使用 '..' 或根绝对路径试图跨越逃逸工作目录。",
			},
			"cwd": map[string]interface{}{
				"type":        "string",
				"description": "执行命令的工作目录，相对于配置的工作目录。留空表示当前工作目录。严禁使用 '..' 或绝对路径逃逸出工作目录。",
			},
		},
		"required": []string{"command"},
	}

	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        ToolName,
			Description: "在系统的终端中执行 Shell 命令或脚本调度指令。支持只读探索与工具调用。只读指令自动执行，修改/高危指令需要用户确认。所有命令与路径必须限制在当前工作目录内部，严禁使用 '..' 或绝对路径跨越/逃逸工作目录。",
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

	cleanWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("工作目录解析失败: %v", err)
	}

	// 1. 校验 cwd 路径边界
	targetDir := cleanWorkDir
	if args.Cwd != "" {
		cleanCwd := filepath.Join(cleanWorkDir, args.Cwd)
		absCwd, err := filepath.Abs(cleanCwd)
		if err != nil || ValidateInWorkDir(absCwd, cleanWorkDir) != nil {
			return "", fmt.Errorf("禁止跨工作目录路径逃逸: %s", args.Cwd)
		}
		targetDir = absCwd
	}

	// 2. 校验 command 中是否包含越界逃逸路径
	if err := ValidateCommandBoundary(cleanWorkDir, command); err != nil {
		return "", fmt.Errorf("禁止跨工作目录路径逃逸: %v", err)
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

	err = cmd.Run()
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
