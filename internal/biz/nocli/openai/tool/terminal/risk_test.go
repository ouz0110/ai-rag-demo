package terminal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsDangerousCommand(t *testing.T) {
	workDir := filepath.Join(os.TempDir(), "workspace", "demo")
	_ = os.MkdirAll(workDir, 0755)
	defer os.RemoveAll(filepath.Join(os.TempDir(), "workspace"))

	safeCmds := []string{
		"ls",
		"dir",
		"pwd",
		"git status",
		"git diff",
		"cat README.md",
		"curl -sL https://example.com",
	}

	for _, cmd := range safeCmds {
		if IsDangerousCommand(workDir, cmd) {
			t.Errorf("期望安全指令放行，实际判定为危险: %s", cmd)
		}
	}

	dangerCmds := []string{
		"rm -rf /",
		"del /f file.txt",
		"git commit -m 'feat'",
		"git push origin main",
		"npm install",
		"go run main.go",
		"echo hello > test.txt",
		"ls -la /Users/oz/code/ringkol/api-rag-demo/workspace-rm-demo",
		"rm -rf ../../workspace-rm-demo",
	}

	for _, cmd := range dangerCmds {
		if !IsDangerousCommand(workDir, cmd) {
			t.Errorf("期望高危指令拦截，实际判定为安全: %s", cmd)
		}
	}
}
