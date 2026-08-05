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

	"ai-rag-demo/internal/biz/nocli/openai/tool/toolutil"
	"ai-rag-demo/internal/common"
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

func (t *Tool) RequiresApproval(ctx context.Context, argsJSON string) bool {
	var args Args
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return true
	}

	baseWorkDir := "./workspace/agent"
	var allowedPaths []string
	if t.cfg != nil && t.cfg.Source.Nocli != nil {
		if t.cfg.Source.Nocli.WorkDir != "" {
			baseWorkDir = t.cfg.Source.Nocli.WorkDir
		}
		allowedPaths = t.cfg.Source.Nocli.AllowedPaths
	}

	workDir, err := common.GetStrictUserAgentWorkDir(ctx, baseWorkDir)
	if err != nil {
		return true
	}

	// 1. 任何企图逃逸工作目录及白名单路径范围的 cwd 或 command 均属于风险/越界操作，强制触发人工审批
	if toolutil.HasPathEscape(workDir, args.Cwd, allowedPaths) || toolutil.HasPathEscape(workDir, args.Command, allowedPaths) {
		return true
	}

	return IsDangerousCommand(workDir, args.Command, allowedPaths)
}

func (t *Tool) Definition() openai.Tool {
	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "要在终端 Shell 中执行的命令。支持包含 pipeline、重定向及多条语句组合。",
			},
			"cwd": map[string]interface{}{
				"type":        "string",
				"description": "命令执行的目标工作子目录（可选，相对于根工作目录。默认在工作目录根节点执行）。",
			},
		},
		"required": []string{"command"},
	}

	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        ToolName,
			Description: "在系统的 zsh 命令行中为用户提案并执行命令。可以用于创建目录、执行编译指令、文件搜索定位或安全的环境工具调用。",
			Parameters:  parameters,
		},
	}
}

func (t *Tool) Run(ctx context.Context, argsJSON string) (string, error) {
	var args Args
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("参数解析失败: %v", err)
	}

	command := strings.TrimSpace(args.Command)
	if command == "" {
		return "", fmt.Errorf("执行命令不能为空")
	}

	baseWorkDir := "./workspace/agent"
	var allowedPaths []string
	if t.cfg != nil && t.cfg.Source.Nocli != nil {
		if t.cfg.Source.Nocli.WorkDir != "" {
			baseWorkDir = t.cfg.Source.Nocli.WorkDir
		}
		allowedPaths = t.cfg.Source.Nocli.AllowedPaths
	}

	cleanWorkDir, err := common.GetStrictUserAgentWorkDir(ctx, baseWorkDir)
	if err != nil {
		return "", fmt.Errorf("解析用户 Agent 工作目录失败: %w", err)
	}

	// 1. 校验 cwd 路径边界
	targetDir := cleanWorkDir
	if args.Cwd != "" {
		var cleanCwd string
		if filepath.IsAbs(args.Cwd) {
			cleanCwd = filepath.Clean(args.Cwd)
		} else {
			cleanCwd = filepath.Clean(filepath.Join(cleanWorkDir, args.Cwd))
		}
		absCwd, err := filepath.Abs(cleanCwd)
		if err != nil || toolutil.ValidateInWorkDirOrAllowed(absCwd, cleanWorkDir, allowedPaths) != nil {
			return "", fmt.Errorf("禁止跨工作目录及许可范围外的路径逃逸: %s", args.Cwd)
		}
		targetDir = absCwd
	}

	// 2. 校验 command 中是否包含越界逃逸路径
	if err := toolutil.ValidateCommandBoundary(cleanWorkDir, command, allowedPaths); err != nil {
		return "", fmt.Errorf("禁止跨工作目录路径逃逸: %v", err)
	}

	execTimeout := 5 * time.Minute
	if t.cfg != nil && t.cfg.Source.Nocli != nil && t.cfg.Source.Nocli.ExecTimeout.Duration > 0 {
		execTimeout = t.cfg.Source.Nocli.ExecTimeout.Duration
	}
	execCtx, cancel := context.WithTimeout(ctx, execTimeout)
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
