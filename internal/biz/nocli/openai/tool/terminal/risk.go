package terminal

import (
	"strings"
)

// SafeCommands 白名单纯查询/只读命令前缀
var SafeCommands = []string{
	"ls", "dir", "pwd", "cat", "type", "echo", "grep", "find",
	"head", "tail", "wc", "which", "where", "env", "whoami",
	"curl", "wget", "git status", "git log", "git diff", "git show",
	"go version", "node -v", "python --version", "python3 --version",
}

// DangerousCommands 明确拦截的高危/修改指令前缀
var DangerousCommands = []string{
	"rm", "del", "erase", "mv", "move", "chmod", "chown",
	"git commit", "git push", "git checkout", "git reset", "git rebase", "git merge",
	"go run", "go build", "make", "npm install", "yarn add", "pip install",
	"sudo", "su", "systemctl", "service", "kill", "pkill", "shutdown", "reboot",
	"powershell", "cmd", "bash", "sh",
}

// IsDangerousCommand 评估终端 Shell 指令是否高危（需触发人工审批中断）
func IsDangerousCommand(workDir, command string) bool {
	cmd := strings.TrimSpace(strings.ToLower(command))
	if cmd == "" {
		return false
	}

	// 0. 优先检测路径逃逸/越界字符 (如含有超出 workDir 的路径引用)
	if HasPathEscape(workDir, command) {
		return true
	}

	// 1. 优先校验明确的高危指令
	for _, dang := range DangerousCommands {
		if cmd == dang || strings.HasPrefix(cmd, dang+" ") || strings.Contains(cmd, " "+dang+" ") || strings.Contains(cmd, "&& "+dang) || strings.Contains(cmd, "; "+dang) || strings.Contains(cmd, "| "+dang) {
			return true
		}
	}

	// 2. 管道与命令拼接重定向处理 (>, >>, |, &&, ;) 通常涉及状态修改
	if strings.Contains(cmd, ">") || strings.Contains(cmd, "&&") || strings.Contains(cmd, ";") {
		return true
	}

	// 3. 校验白名单安全只读指令
	for _, safe := range SafeCommands {
		if cmd == safe || strings.HasPrefix(cmd, safe+" ") {
			return false
		}
	}

	// 默认未明确白名单放行的命令均视作高危指令，触发安全拦截
	return true
}
