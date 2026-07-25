package agent

import (
	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	"sync"
)

type Registry struct {
	mu     sync.RWMutex
	agents map[string]base.IAgent
}

func NewRegistry(baseAgent *base.BaseAgent) *Registry {
	r := &Registry{
		agents: make(map[string]base.IAgent),
	}
	r.Register(NewMainAgent(baseAgent))
	r.Register(NewFileAnalyzerAgent(baseAgent))
	r.Register(NewRAGAgent(baseAgent))
	return r
}

func (r *Registry) Register(agent base.IAgent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agent.Name()] = agent
}

func (r *Registry) Get(name string) (base.IAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ag, ok := r.agents[name]
	return ag, ok
}

func (r *Registry) List() []base.IAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]base.IAgent, 0, len(r.agents))
	for _, ag := range r.agents {
		list = append(list, ag)
	}
	return list
}
