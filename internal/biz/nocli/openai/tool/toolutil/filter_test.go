package toolutil

import (
	"ai-rag-demo/internal/conf"
	"testing"
)

func TestFilterUtils(t *testing.T) {
	cfg := &conf.Config{}
	cfg.Source.Nocli = &conf.NocliConfig{
		IgnoredPaths:    []string{"custom_build"},
		AllowedSuffixes: []string{".go", ".md"},
	}

	ParseFilters(cfg)

	// 测试忽略路径
	if !ShouldIgnore(".git") {
		t.Errorf("期望忽略 .git，实际未忽略")
	}
	if !ShouldIgnore("custom_build") {
		t.Errorf("期望忽略 custom_build，实际未忽略")
	}
	if ShouldIgnore("main.go") {
		t.Errorf("不期望忽略 main.go")
	}

	// 测试允许后缀
	if !IsAllowedFile("main.go") {
		t.Errorf("期望许可 .go 文件，实际未许可")
	}
	if !IsAllowedFile("README.md") {
		t.Errorf("期望许可 .md 文件，实际未许可")
	}
	if IsAllowedFile("image.png") {
		t.Errorf("期望拒绝 .png 文件")
	}
}
