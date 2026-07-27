package terminal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasPathEscape(t *testing.T) {
	workDir := filepath.Join(os.TempDir(), "workspace", "demo")
	_ = os.MkdirAll(workDir, 0755)
	defer os.RemoveAll(filepath.Join(os.TempDir(), "workspace"))

	// 兄弟目录，不在 workDir 内部
	siblingDir := filepath.Join(os.TempDir(), "workspace-rm-demo")

	escapeCmds := []string{
		// 用户提报的具体案例：访问同级/兄弟目录
		"ls -la " + siblingDir,
		"rm -rf " + siblingDir,
		// 相对路径逃逸
		"ls -al ../../ | grep -i workspace",
		"rm -rf ../../workspace-rm-demo",
		"cat ../secret.txt",
		"ls ..",
		"cd ../",
		"ls demo/../../workspace-rm-demo",
		"echo test > ../test.txt",
		// 根路径与家目录逃逸
		"cat /etc/passwd",
		"cat ~/secret.txt",
		"python --input=/var/log/syslog",
		"cp /etc/hosts ./",
	}

	for _, cmd := range escapeCmds {
		if !HasPathEscape(workDir, cmd) {
			t.Errorf("期望检测到路径逃逸，实际判定安全: %s", cmd)
		}
		if err := ValidateCommandBoundary(workDir, cmd); err == nil {
			t.Errorf("期望 ValidateCommandBoundary 拦截逃逸，实际通过: %s", cmd)
		}
	}

	normalCmds := []string{
		"ls -la",
		"cat ./main.go",
		"git status",
		"git log main..feature",
		"grep -rn 'func' ./src",
		"echo 'hello world'",
	}

	for _, cmd := range normalCmds {
		if HasPathEscape(workDir, cmd) {
			t.Errorf("误判正常命令为路径逃逸: %s", cmd)
		}
		if err := ValidateCommandBoundary(workDir, cmd); err != nil {
			t.Errorf("正常命令被拦截: %s, err: %v", cmd, err)
		}
	}
}
