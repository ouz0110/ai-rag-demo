package list_file

import (
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	ignoredPaths   map[string]struct{}
	allowedSuffixes map[string]struct{}
	filtersOnce     sync.Once
)

func ParseFilters(cfg *conf.Config) (ignored map[string]struct{}, allowed map[string]struct{}) {
	filtersOnce.Do(func() {
		defaultIgnored := []string{
			".git", ".svn", ".hg",
			".idea", ".vscode", ".vs",
			"node_modules", "vendor",
			"dist", "build", "bin", "out",
			"tmp", "temp", "logs",
			".cache", "__pycache__",
			".DS_Store", "Thumbs.db",
			".gitignore", ".gitattributes",
		}
		ignoredPaths = make(map[string]struct{}, len(defaultIgnored)+len(cfg.Source.Nocli.IgnoredPaths))
		for _, p := range defaultIgnored {
			ignoredPaths[p] = struct{}{}
		}
		for _, p := range cfg.Source.Nocli.IgnoredPaths {
			ignoredPaths[p] = struct{}{}
		}

		allowedSuffixes = make(map[string]struct{}, len(cfg.Source.Nocli.AllowedSuffixes))
		for _, s := range cfg.Source.Nocli.AllowedSuffixes {
			allowedSuffixes[s] = struct{}{}
		}
	})
	return ignoredPaths, allowedSuffixes
}

type ResolveResult struct {
	WorkDir  string
	Target   string
	RelPath  string
}

func ResolvePath(cfg *conf.Config, userPath string) (*ResolveResult, error) {
	return ResolvePathWithCtx(context.Background(), cfg, userPath)
}

func ResolvePathWithCtx(ctx context.Context, cfg *conf.Config, userPath string) (*ResolveResult, error) {
	baseWorkDir := "./workspace/agent"
	if cfg != nil && cfg.Source.Nocli != nil && cfg.Source.Nocli.WorkDir != "" {
		baseWorkDir = cfg.Source.Nocli.WorkDir
	}

	cleanWorkDir, err := common.GetStrictUserAgentWorkDir(ctx, baseWorkDir)
	if err != nil {
		return nil, fmt.Errorf("解析用户 Agent 工作目录失败: %w", err)
	}

	target := filepath.Join(cleanWorkDir, userPath)
	cleanTarget, err := filepath.Abs(target)
	if err != nil {
		return nil, fmt.Errorf("路径解析失败: %v", err)
	}

	if err := ValidateInWorkDir(cleanTarget, cleanWorkDir); err != nil {
		return nil, err
	}

	relPath, err := filepath.Rel(cleanWorkDir, cleanTarget)
	if err != nil {
		return nil, fmt.Errorf("路径计算失败: %v", err)
	}

	return &ResolveResult{
		WorkDir: cleanWorkDir,
		Target:  cleanTarget,
		RelPath: relPath,
	}, nil
}

func ValidateInWorkDir(target, workDir string) error {
	if target == workDir {
		return nil
	}

	rel, err := filepath.Rel(workDir, target)
	if err != nil {
		return fmt.Errorf("路径校验失败: %v", err)
	}

	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("禁止访问工作目录外的路径: %s", target)
	}

	return nil
}

func ShouldIgnore(name string) bool {
	if len(ignoredPaths) == 0 {
		return false
	}
	base := filepath.Base(name)
	_, ok := ignoredPaths[base]
	return ok
}

func IsAllowedFile(name string) bool {
	if len(allowedSuffixes) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	_, ok := allowedSuffixes[ext]
	return ok
}

func OpenDir(path string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败 %s: %v", path, err)
	}
	return entries, nil
}
