package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Registry struct {
	skillsDir string
	mu        sync.RWMutex
	skills    map[string]*Skill
}

func NewRegistry(skillsDir string) *Registry {
	return &Registry{
		skillsDir: skillsDir,
		skills:    make(map[string]*Skill),
	}
}

// Scan 自动扫描技能根目录下的所有子目录并解析加载 SKILL.md
func (r *Registry) Scan() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.skillsDir == "" {
		return nil
	}

	entries, err := os.ReadDir(r.skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取技能目录失败 [%s]: %v", r.skillsDir, err)
	}

	newSkills := make(map[string]*Skill)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillFile := filepath.Join(r.skillsDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			continue
		}

		s, err := ParseFile(skillFile)
		if err != nil {
			// 单个 Skill 解析失败不中断整体扫描，打印日志跳过
			fmt.Printf("[SkillRegistry] 忽略非法 Skill [%s]: %v\n", skillFile, err)
			continue
		}

		newSkills[s.Frontmatter.Name] = s
		fmt.Println("load skill: ", s.Frontmatter.Name)
	}

	r.skills = newSkills
	return nil
}

func (r *Registry) GetSkill(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.skills[name]
	return s, ok
}

func (r *Registry) ListSkills() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		list = append(list, s)
	}
	return list
}
