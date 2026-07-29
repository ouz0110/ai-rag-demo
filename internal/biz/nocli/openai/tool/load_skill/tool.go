package loadskill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/skill"

	openai "github.com/sashabaranov/go-openai"
)

const ToolName = "load_skill"

type Args struct {
	Name string `json:"name"`
}

type Tool struct {
	cfg      *conf.Config
	skillMgr *skill.Manager
}

func NewTool(cfg *conf.Config, skillMgr *skill.Manager) *Tool {
	return &Tool{
		cfg:      cfg,
		skillMgr: skillMgr,
	}
}

func (t *Tool) RequiresApproval(ctx context.Context, argsJSON string) bool {
	return false
}

func (t *Tool) Definition() openai.Tool {
	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "要装载的技能名称 (name)，必须为技能元数据列表中声明的技能名称。",
			},
		},
		"required": []string{"name"},
	}

	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        ToolName,
			Description: "动态装载并读取指定技能 (Skill) 的完整 SOP 操作指引与规范说明。当用户意图匹配某种技能时，必须调用此工具传入技能名称，系统将实时返回该技能最新的 SOP Markdown 规范。",
			Parameters:  parameters,
		},
	}
}

func (t *Tool) Run(ctx context.Context, argsJSON string) (string, error) {
	if t.skillMgr == nil {
		return "", fmt.Errorf("技能管理器 (skillManager) 未初始化")
	}

	var args Args
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("参数解析失败: %v", err)
	}

	skillName := strings.TrimSpace(args.Name)
	if skillName == "" {
		return "", fmt.Errorf("技能名称 name 不能为空")
	}

	s, ok := t.skillMgr.GetLatestSkill(skillName)
	if !ok || s == nil {
		availableSkills := t.skillMgr.ListAvailableSkillNames()
		return "", fmt.Errorf("未找到名为 [%s] 的技能，当前可用技能列表: %v", skillName, availableSkills)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== 🎯 技能 SOP 操作规范装载成功: %s ===\n", s.Frontmatter.Name))
	sb.WriteString(fmt.Sprintf("描述: %s\n", s.Frontmatter.Description))
	sb.WriteString(fmt.Sprintf("根路径: %s\n\n", s.Path))
	sb.WriteString("--- SOP 指引内容 ---\n")
	sb.WriteString(s.Body)
	sb.WriteString("\n--- END SOP ---\n")

	return sb.String(), nil
}
