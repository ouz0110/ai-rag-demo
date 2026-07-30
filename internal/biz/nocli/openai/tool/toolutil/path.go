package toolutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
)

// Standard system binary directory prefixes that can appear at the command executable position
var systemBinaryPrefixes = []string{
	"/bin/", "/usr/bin/", "/usr/local/bin/", "/sbin/", "/usr/sbin/",
	`c:\windows\system32\`, `c:\windows\`,
}

// ResolveResult 封装路径解析与白名单校验后的结果实体
type ResolveResult struct {
	WorkDir string // Agent 对应的根工作目录绝对路径
	Target  string // 目标文件或目录的绝对路径
	RelPath string // 可读展示路径 (若在 WorkDir 内为相对路径，若在外部白名单则为绝对路径)
}

// ----------------------------------------------------------------------------
// 1. 核心路径层级与白名单比对
// ----------------------------------------------------------------------------

// IsSubPath 检查 target 路径是否位于 parent 路径内部或与其一致
func IsSubPath(target, parent string) bool {
	if target == parent {
		return true
	}
	rel, err := filepath.Rel(parent, target)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

// ValidateInWorkDirOrAllowed 校验 target 路径是否位于 workDir 内部或任一 allowedPaths 白名单绝对路径下
func ValidateInWorkDirOrAllowed(target, workDir string, allowedPaths []string) error {
	cleanTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("路径解析失败: %v", err)
	}

	// 1. 校验是否在 workDir 内
	if workDir != "" {
		if cleanWorkDir, err := filepath.Abs(workDir); err == nil {
			if IsSubPath(cleanTarget, cleanWorkDir) {
				return nil
			}
		}
	}

	// 2. 校验是否在 allowedPaths 任意白名单绝对路径内
	for _, allowed := range allowedPaths {
		if strings.TrimSpace(allowed) == "" {
			continue
		}
		// 📌 强制要求：所有配置的白名单路径必须转为绝对路径
		cleanAllowed, err := filepath.Abs(allowed)
		if err == nil {
			if IsSubPath(cleanTarget, cleanAllowed) {
				return nil // 命中白名单，校验通过！
			}
		}
	}

	return fmt.Errorf("禁止访问许可范围外的路径: %s", cleanTarget)
}

// ----------------------------------------------------------------------------
// 2. 上下文路径解析与归一化
// ----------------------------------------------------------------------------

// ResolvePath 快捷解析 userPath 为标准绝对路径实体 (兼容无 Context 场景)
func ResolvePath(ctx context.Context, cfg *conf.Config, userPath string) (*ResolveResult, error) {
	return ResolvePathWithCtx(ctx, cfg, userPath)
}

// ResolvePathWithCtx 结合 Context 与全量 Config 动态解析 userPath 并做白名单安全防护
func ResolvePathWithCtx(ctx context.Context, cfg *conf.Config, userPath string) (*ResolveResult, error) {
	baseWorkDir := "./workspace/agent"
	var allowedPaths []string
	if cfg != nil && cfg.Source.Nocli != nil {
		if cfg.Source.Nocli.WorkDir != "" {
			baseWorkDir = cfg.Source.Nocli.WorkDir
		}
		allowedPaths = cfg.Source.Nocli.AllowedPaths
	}

	cleanWorkDir, err := common.GetStrictUserAgentWorkDir(ctx, baseWorkDir)
	if err != nil {
		return nil, fmt.Errorf("解析用户 Agent 工作目录失败: %w", err)
	}

	var cleanTarget string
	if filepath.IsAbs(userPath) {
		cleanTarget = filepath.Clean(userPath)
	} else {
		cleanTarget = filepath.Clean(filepath.Join(cleanWorkDir, userPath))
	}

	if err := ValidateInWorkDirOrAllowed(cleanTarget, cleanWorkDir, allowedPaths); err != nil {
		return nil, err
	}

	relPath, err := filepath.Rel(cleanWorkDir, cleanTarget)
	if err != nil || strings.HasPrefix(relPath, "..") {
		relPath = cleanTarget
	}

	return &ResolveResult{
		WorkDir: cleanWorkDir,
		Target:  cleanTarget,
		RelPath: relPath,
	}, nil
}

// ----------------------------------------------------------------------------
// 3. Shell 命令路径提取与越界逃逸防范
// ----------------------------------------------------------------------------

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

// HasPathEscape 检查命令或路径中是否包含超出 workDir 及 allowedPaths 白名单的越界/逃逸特征
func HasPathEscape(workDir, cmdOrPath string, allowedPaths ...[]string) bool {
	if strings.TrimSpace(cmdOrPath) == "" {
		return false
	}
	err := ValidateCommandBoundary(workDir, cmdOrPath, allowedPaths...)
	return err != nil
}

// ValidateCommandBoundary 校验命令及其包含的所有路径 Candidate 是否包含超出工作目录及白名单范围的越界路径
func ValidateCommandBoundary(workDir, command string, allowedPaths ...[]string) error {
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

	var allowed []string
	if len(allowedPaths) > 0 {
		allowed = allowedPaths[0]
	}

	candidates := ExtractPathCandidates(cmd)
	for _, candidate := range candidates {
		var absTarget string
		if filepath.IsAbs(candidate) {
			absTarget = filepath.Clean(candidate)
		} else {
			absTarget = filepath.Clean(filepath.Join(cleanWorkDir, candidate))
		}

		if err := ValidateInWorkDirOrAllowed(absTarget, cleanWorkDir, allowed); err != nil {
			return fmt.Errorf("指令中包含超出许可目录范围的越界路径 '%s'", candidate)
		}
	}

	return nil
}
