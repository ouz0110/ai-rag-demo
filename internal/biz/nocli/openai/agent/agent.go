package agent

import "ai-rag-demo/internal/pkg/skill"

// Agent 定义通用的 Agent 接口
type Agent interface {
	Name() string
	Description() string
	SystemPrompt(workDir string, skillMgr *skill.Manager) string
}

// Registry 管理项目中注册的所有 Agent
type Registry struct {
	agents map[string]Agent
}

func NewRegistry() *Registry {
	r := &Registry{
		agents: make(map[string]Agent),
	}
	r.Register(NewMainAgent())
	r.Register(NewCodeAnalyzerAgent())
	return r
}

func (r *Registry) Register(a Agent) {
	r.agents[a.Name()] = a
}

func (r *Registry) Get(name string) (Agent, bool) {
	a, ok := r.agents[name]
	return a, ok
}

func (r *Registry) List() []Agent {
	list := make([]Agent, 0, len(r.agents))
	for _, a := range r.agents {
		list = append(list, a)
	}
	return list
}
