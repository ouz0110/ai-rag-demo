package toolutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ai-rag-demo/internal/conf"
)

var (
	ignoredPaths    map[string]struct{}
	allowedSuffixes map[string]struct{}
	filtersOnce     sync.Once
)

// ParseFilters 解析全局 Config 中的忽略路径及允许扩展名配置 (全局解析单例)
func ParseFilters(cfg *conf.Config) (ignored map[string]struct{}, allowed map[string]struct{}) {
	if cfg == nil || cfg.Source.Nocli == nil {
		return ignoredPaths, allowedSuffixes
	}

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

// ShouldIgnore 判断文件或目录名是否在忽略列表中
func ShouldIgnore(name string) bool {
	if len(ignoredPaths) == 0 {
		return false
	}
	base := filepath.Base(name)
	_, ok := ignoredPaths[base]
	return ok
}

// IsAllowedFile 判断文件后缀名是否在许可类型列表中
func IsAllowedFile(name string) bool {
	if len(allowedSuffixes) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	_, ok := allowedSuffixes[ext]
	return ok
}

// OpenDir 打开并读取目标目录下的所有文件条目
func OpenDir(path string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败 %s: %v", path, err)
	}
	return entries, nil
}
