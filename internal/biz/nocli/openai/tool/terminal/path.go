package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Standard system binary directory prefixes that can appear at the command executable position
var systemBinaryPrefixes = []string{
	"/bin/", "/usr/bin/", "/usr/local/bin/", "/sbin/", "/usr/sbin/",
	`c:\windows\system32\`, `c:\windows\`,
}

// ValidateInWorkDir 检查目标路径 target 是否完全位于 workDir 内部或与其等价
func ValidateInWorkDir(target, workDir string) error {
	cleanTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("无效的目标路径: %v", err)
	}
	cleanWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return fmt.Errorf("无效的工作目录: %v", err)
	}

	if cleanTarget == cleanWorkDir {
		return nil
	}

	rel, err := filepath.Rel(cleanWorkDir, cleanTarget)
	if err != nil {
		return fmt.Errorf("计算相对路径失败: %v", err)
	}

	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("目标路径 '%s' 超出工作目录范围 '%s'", cleanTarget, cleanWorkDir)
	}

	return nil
}

// ExtractPathCandidates 从 Shell 命令字符串中解析并提取所有可能的目标路径 Candidate
func ExtractPathCandidates(command string) []string {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return nil
	}

	// 替换环境变量 $HOME 和 ~ 为绝对路径
	homeDir, err := os.UserHomeDir()
	if err == nil && homeDir != "" {
		cmd = strings.ReplaceAll(cmd, "$HOME", homeDir)
		cmd = strings.ReplaceAll(cmd, "${HOME}", homeDir)
	}

	// 按 Shell 分隔符拆分为 tokens
	tokens := strings.FieldsFunc(cmd, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
			r == '|' || r == '&' || r == ';' || r == '>' || r == '<' ||
			r == '"' || r == '\'' || r == '`' || r == '=' ||
			r == '(' || r == ')' || r == '{' || r == '}'
	})

	var candidates []string
	for i, token := range tokens {
		token = strings.Trim(token, " \t\r\n`'\"")
		if token == "" {
			continue
		}

		// 扩展波浪号 ~
		if strings.HasPrefix(token, "~/") || token == "~" {
			if homeDir != "" {
				token = filepath.Join(homeDir, token[1:])
			}
		}

		// 跳过常见的纯选项 Flag（如 -la, -rf, --help, -v 等，无路径分隔符或点号）
		if strings.HasPrefix(token, "-") && !strings.Contains(token, "/") && !strings.Contains(token, "\\") && !strings.Contains(token, "..") {
			continue
		}

		// 系统可执行文件路径特例 (如位于命令首位的 /bin/ls, /usr/bin/git)
		if i == 0 && filepath.IsAbs(token) {
			lowerToken := strings.ToLower(filepath.ToSlash(token))
			isSysBin := false
			for _, sysPrefix := range systemBinaryPrefixes {
				if strings.HasPrefix(lowerToken, strings.ToLower(sysPrefix)) {
					isSysBin = true
					break
				}
			}
			if isSysBin {
				continue
			}
		}

		// 包含了路径修饰词 (/ , \ , . , ~) 或单独单词均提取作为路径候选
		candidates = append(candidates, token)
	}

	return candidates
}

// HasPathEscape 检查命令或路径中是否包含超出 workDir 的越界/逃逸特征
func HasPathEscape(workDir, cmdOrPath string) bool {
	if strings.TrimSpace(cmdOrPath) == "" {
		return false
	}
	err := ValidateCommandBoundary(workDir, cmdOrPath)
	return err != nil
}

// ValidateCommandBoundary 校验命令及其包含的所有路径 Candidate 是否完全限制在工作目录内部
func ValidateCommandBoundary(workDir, command string) error {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return nil
	}

	if workDir == "" {
		workDir = "."
	}

	cleanWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return fmt.Errorf("工作目录解析失败: %v", err)
	}

	candidates := ExtractPathCandidates(cmd)
	for _, candidate := range candidates {
		var absTarget string
		if filepath.IsAbs(candidate) {
			absTarget = filepath.Clean(candidate)
		} else {
			absTarget = filepath.Clean(filepath.Join(cleanWorkDir, candidate))
		}

		if err := ValidateInWorkDir(absTarget, cleanWorkDir); err != nil {
			return fmt.Errorf("指令中包含超出工作目录范围的越界路径 '%s'", candidate)
		}
	}

	return nil
}
