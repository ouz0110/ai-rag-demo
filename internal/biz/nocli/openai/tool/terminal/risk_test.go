package terminal

import (
	"testing"
)

func TestIsDangerousCommand(t *testing.T) {
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
		if IsDangerousCommand(cmd) {
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
	}

	for _, cmd := range dangerCmds {
		if !IsDangerousCommand(cmd) {
			t.Errorf("期望高危指令拦截，实际判定为安全: %s", cmd)
		}
	}
}
