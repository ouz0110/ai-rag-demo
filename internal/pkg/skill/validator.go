package skill

import (
	"fmt"
	"regexp"
	"strings"
)

var nameRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Validate 校验 Frontmatter 是否符合 agentskills.io 规范
func Validate(fm *Frontmatter) error {
	if fm == nil {
		return fmt.Errorf("frontmatter 不能为空")
	}

	name := strings.TrimSpace(fm.Name)
	if len(name) < 1 || len(name) > 64 {
		return fmt.Errorf("skill 名称长度必须在 1 到 64 个字符之间，当前: %d", len(name))
	}

	if !nameRegexp.MatchString(name) {
		return fmt.Errorf("skill 名称格式非法: %s (仅允许小写字母、数字及单连字符，不能以连字符开头或结尾)", name)
	}

	desc := strings.TrimSpace(fm.Description)
	if len(desc) < 1 || len(desc) > 1024 {
		return fmt.Errorf("skill 描述长度必须在 1 到 1024 个字符之间，当前: %d", len(desc))
	}

	return nil
}
