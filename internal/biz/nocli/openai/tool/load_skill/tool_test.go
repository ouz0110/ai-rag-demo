package loadskill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-rag-demo/internal/pkg/skill"
)

func TestLoadSkillTool(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "demo-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("创建临时技能目录失败: %v", err)
	}

	skillContentV1 := `---
name: demo-skill
description: 演示技能 V1。
---
# SOP V1: 初始步骤
1. 执行第一步
`
	skillFilePath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFilePath, []byte(skillContentV1), 0644); err != nil {
		t.Fatalf("写入 SKILL.md 失败: %v", err)
	}

	reg := skill.NewRegistry(tmpDir)
	if err := reg.Scan(); err != nil {
		t.Fatalf("Scan 技能目录失败: %v", err)
	}
	mgr := skill.NewManager(reg)

	tool := NewTool(nil, mgr)

	// 1. 测试第一次装载 SOP
	res, err := tool.Run(context.Background(), `{"name": "demo-skill"}`)
	if err != nil {
		t.Fatalf("调用 load_skill 失败: %v", err)
	}
	if !strings.Contains(res, "SOP V1: 初始步骤") {
		t.Errorf("返回内容期望包含 V1 SOP, 实际: %s", res)
	}

	// 2. 测试修改磁盘 SKILL.md 后的实时热装载 (Dynamic Reload)
	skillContentV2 := `---
name: demo-skill
description: 演示技能 V2 更新版。
---
# SOP V2: 修改后的最新步骤
1. 执行最新更新后的第一步
2. 执行第二步
`
	if err := os.WriteFile(skillFilePath, []byte(skillContentV2), 0644); err != nil {
		t.Fatalf("更新 SKILL.md 失败: %v", err)
	}

	resV2, err := tool.Run(context.Background(), `{"name": "demo-skill"}`)
	if err != nil {
		t.Fatalf("第二次调用 load_skill 失败: %v", err)
	}
	if !strings.Contains(resV2, "SOP V2: 修改后的最新步骤") {
		t.Errorf("返回内容期望包含最新 V2 SOP, 实际: %s", resV2)
	}

	// 3. 测试不存在的技能名称
	_, errNotfound := tool.Run(context.Background(), `{"name": "not-exist"}`)
	if errNotfound == nil {
		t.Errorf("期望不存在的技能报错, 实际未报错")
	}
}
