package skill

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseFile 解析指定 SKILL.md 文件并返回 Skill 实体
func ParseFile(filePath string) (*Skill, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取 SKILL.md 失败: %v", err)
	}

	skill, err := ParseContent(content)
	if err != nil {
		return nil, fmt.Errorf("解析 SKILL.md [%s] 失败: %v", filePath, err)
	}

	absPath, err := filepath.Abs(filePath)
	if err == nil {
		filePath = absPath
	}

	skill.Path = filepath.Dir(filePath)
	return skill, nil
}

// ParseContent 解析 SKILL.md 的 raw bytes 内容，分离 Frontmatter 与 Markdown Body
func ParseContent(content []byte) (*Skill, error) {
	str := string(bytes.TrimSpace(content))

	if !strings.HasPrefix(str, "---") {
		return nil, fmt.Errorf("格式非法: 缺失 Frontmatter 分隔符 ---")
	}

	// 寻找第二个 --- 分隔符
	rest := str[3:]
	idx := strings.Index(rest, "---")
	if idx == -1 {
		return nil, fmt.Errorf("格式非法: Frontmatter 未关闭 (未找到闭合 ---)")
	}

	yamlContent := rest[:idx]
	markdownBody := strings.TrimSpace(rest[idx+3:])

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return nil, fmt.Errorf("YAML Frontmatter 解析失败: %v", err)
	}

	if err := Validate(&fm); err != nil {
		return nil, err
	}

	return &Skill{
		Frontmatter: fm,
		Body:        markdownBody,
	}, nil
}
