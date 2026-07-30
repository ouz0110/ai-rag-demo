package toolutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateInWorkDirOrAllowed(t *testing.T) {
	tempDir := t.TempDir()

	workDir := filepath.Join(tempDir, "workspace", "agent")
	skillsDir := filepath.Join(tempDir, "workspace", "skills")
	tmpDir := filepath.Join(tempDir, "system_tmp")

	_ = os.MkdirAll(workDir, 0755)
	_ = os.MkdirAll(skillsDir, 0755)
	_ = os.MkdirAll(tmpDir, 0755)

	allowedPaths := []string{
		skillsDir,
		tmpDir,
	}

	// 1. 位于 workDir 内部的测试
	inWorkFile := filepath.Join(workDir, "main.go")
	if err := ValidateInWorkDirOrAllowed(inWorkFile, workDir, allowedPaths); err != nil {
		t.Errorf("期望 WorkDir 内部文件通过校验，实际报错: %v", err)
	}

	// 2. 位于 allowedPaths 白名单内的测试
	inSkillsFile := filepath.Join(skillsDir, "web-fetcher", "SKILL.md")
	if err := ValidateInWorkDirOrAllowed(inSkillsFile, workDir, allowedPaths); err != nil {
		t.Errorf("期望白名单目录下的文件通过校验，实际报错: %v", err)
	}

	inTmpFile := filepath.Join(tmpDir, "cache.json")
	if err := ValidateInWorkDirOrAllowed(inTmpFile, workDir, allowedPaths); err != nil {
		t.Errorf("期望 tmp 白名单目录下的文件通过校验，实际报错: %v", err)
	}

	// 3. 超出 workDir 且未在 allowedPaths 白名单内的非法测试
	illegalFile := filepath.Join(tempDir, "forbidden", "secret.key")
	if err := ValidateInWorkDirOrAllowed(illegalFile, workDir, allowedPaths); err == nil {
		t.Errorf("期望非法越界访问被拒绝，但未报错")
	}
}

func TestHasPathEscapeAndCommandBoundary(t *testing.T) {
	tempDir := t.TempDir()
	workDir := filepath.Join(tempDir, "workspace", "agent")
	allowedDir := filepath.Join(tempDir, "allowed")

	_ = os.MkdirAll(workDir, 0755)
	_ = os.MkdirAll(allowedDir, 0755)

	allowedPaths := []string{allowedDir}

	// 合法指令测试
	validCmds := []string{
		"ls -la",
		"cat main.go",
		"cat " + filepath.Join(allowedDir, "file.txt"),
	}
	for _, cmd := range validCmds {
		if HasPathEscape(workDir, cmd, allowedPaths) {
			t.Errorf("期望命令 [%s] 通过逃逸检查，实际被判定为逃逸", cmd)
		}
	}

	// 越界指令测试
	invalidCmds := []string{
		"cat ../secret.txt",
		"rm -rf " + filepath.Join(tempDir, "forbidden.txt"),
	}
	for _, cmd := range invalidCmds {
		if !HasPathEscape(workDir, cmd, allowedPaths) {
			t.Errorf("期望命令 [%s] 被判定为越界逃逸，实际未触发逃逸警告", cmd)
		}
	}
}
