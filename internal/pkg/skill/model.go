package skill

// Frontmatter 定义 SKILL.md 头部 YAML 元数据结构 (符合 agentskills.io 规范)
type Frontmatter struct {
	Name          string            `yaml:"name" json:"name"`
	Description   string            `yaml:"description" json:"description"`
	License       string            `yaml:"license,omitempty" json:"license,omitempty"`
	Compatibility string            `yaml:"compatibility,omitempty" json:"compatibility,omitempty"`
	AllowedTools  string            `yaml:"allowed-tools,omitempty" json:"allowed_tools,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// Skill 实体对象
type Skill struct {
	Frontmatter Frontmatter `json:"frontmatter"`
	Body        string      `json:"body"` // SKILL.md Markdown 指令部分
	Path        string      `json:"path"` // Skill 所在绝对路径
}
