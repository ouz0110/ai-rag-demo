package agent

import (
	"sync"

	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"ai-rag-demo/internal/biz/nocli/openai/tool"
	"ai-rag-demo/internal/biz/nocli/vector"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/external/mcp"
	"ai-rag-demo/internal/pkg/skill"
)

type Registry struct {
	mu     sync.RWMutex
	agents map[string]base.IAgent
}

// NewRegistry 统一进行多 Agent 的构造与动态 Tool 注入装配
func NewRegistry(cfg *conf.Config, chatModel *chatmodel.ChatModel, skillMgr *skill.Manager, mcpMgr mcp.Manager, engines ...*vector.VectorEngine) *Registry {
	r := &Registry{
		agents: make(map[string]base.IAgent),
	}

	// 1. 初始化全量底层物理工具仓库 (list_files, read_files, terminal, rag_search, load_skill, call_mcp_tool)
	baseTools := tool.NewRegistry(cfg, skillMgr, mcpMgr, engines...)

	// 2. 实例化各个 Agent (各 Agent 会在各自的构造函数内部，显式声明并挑选自己所需的物理工具)
	fileAnalyzer := NewFileAnalyzerAgent(cfg, baseTools)
	ragAgent := NewRAGAgent(cfg, baseTools)
	mainAgent := NewMainAgent(cfg, baseTools, skillMgr, mcpMgr)

	// 4. 为 MainAgent 动态注入 SubAgent 工具 (Agent-as-a-Tool)
	defaultAgentOpts := AgentToolOptions{
		PassFullContextToSubAgent: true,  // 默认不透传父上下文给子
		ReturnFullContextToParent: true,  // 默认不返回子全部上下文给父
		StreamSubAgentExecution:   false, // 默认流式展示子 Agent 执行过程
	}
	mainAgent.RegisterSubAgentTool(fileAnalyzer, chatModel, defaultAgentOpts)
	mainAgent.RegisterSubAgentTool(ragAgent, chatModel, defaultAgentOpts)

	// 5. 将各 Agent 注册进 Agent 注册表
	r.Register(mainAgent)
	r.Register(fileAnalyzer)
	r.Register(ragAgent)

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
