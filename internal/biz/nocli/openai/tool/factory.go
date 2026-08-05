package tool

import (
	"context"
	"fmt"

	"ai-rag-demo/internal/biz/nocli/checkpoint"
	mcptool "ai-rag-demo/internal/biz/nocli/openai/tool/mcp_tool"
	"ai-rag-demo/internal/biz/nocli/vector"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/external/mcp"
	"ai-rag-demo/internal/pkg/skill"

	openai "github.com/sashabaranov/go-openai"

	list_file "ai-rag-demo/internal/biz/nocli/openai/tool/list_files"
	loadskill "ai-rag-demo/internal/biz/nocli/openai/tool/load_skill"
	ragtool "ai-rag-demo/internal/biz/nocli/openai/tool/rag"
	readfiles "ai-rag-demo/internal/biz/nocli/openai/tool/read_files"
	terminaltool "ai-rag-demo/internal/biz/nocli/openai/tool/terminal"
)

type Tool interface {
	Definition() openai.Tool
	Run(ctx context.Context, argsJSON string) (string, error)
	RequiresApproval(ctx context.Context, argsJSON string) bool
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry(cfg *conf.Config, skillMgr *skill.Manager, mcpMgr mcp.Manager, engines ...*vector.VectorEngine) *Registry {
	var vecEngine *vector.VectorEngine
	if len(engines) > 0 {
		vecEngine = engines[0]
	}

	m := map[string]Tool{
		list_file.ToolName:    list_file.NewTool(cfg),
		readfiles.ToolName:    readfiles.NewTool(cfg),
		terminaltool.ToolName: terminaltool.NewTool(cfg),
	}

	// 📌 仅当 RAG 全局配置开启时才挂载 rag_search 工具
	if cfg != nil && cfg.Source.RAG != nil && cfg.Source.RAG.Enable {
		m[ragtool.ToolName] = ragtool.NewTool(cfg, vecEngine)
	}

	// 📌 仅当 Skill 全局配置开启时才挂载 load_skill 工具
	if skillMgr != nil && cfg != nil && cfg.Source.Skill != nil && cfg.Source.Skill.Enable {
		m[loadskill.ToolName] = loadskill.NewTool(cfg, skillMgr)
	}

	// 📌 仅当 MCP 全局配置开启且 mcpMgr 不为空时挂载 call_mcp_tool 通用执行工具
	if mcpMgr != nil && cfg != nil && cfg.Source.MCP != nil && cfg.Source.MCP.Enable {
		m[mcptool.ToolName] = mcptool.NewTool(cfg, mcpMgr)
	}

	return &Registry{
		tools: m,
	}
}

func NewEmptyRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Clone() *Registry {
	cloned := make(map[string]Tool, len(r.tools))
	for k, v := range r.tools {
		cloned[k] = v
	}
	return &Registry{tools: cloned}
}

func (r *Registry) Filter(names ...string) *Registry {
	filtered := make(map[string]Tool)
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for name, t := range r.tools {
		if nameSet[name] {
			filtered[name] = t
		}
	}
	return &Registry{tools: filtered}
}

func (r *Registry) Register(t Tool) {
	if r.tools == nil {
		r.tools = make(map[string]Tool)
	}
	r.tools[t.Definition().Function.Name] = t
}

func (r *Registry) BuildTools() []openai.Tool {
	defs := make([]openai.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

func (r *Registry) Call(ctx context.Context, name, argsJSON string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("未知工具: %s", name)
	}
	return t.Run(ctx, argsJSON)
}

func (r *Registry) RequiresApproval(ctx context.Context, name, argsJSON string) bool {
	t, ok := r.tools[name]
	if !ok {
		return false
	}
	return t.RequiresApproval(ctx, argsJSON)
}

type CheckpointStoreSetter interface {
	SetCheckpointStore(checkpoint.ICheckpointStore)
}

func (r *Registry) SetCheckpointStore(store checkpoint.ICheckpointStore) {
	if r == nil {
		return
	}
	for _, t := range r.tools {
		if setter, ok := t.(CheckpointStoreSetter); ok {
			setter.SetCheckpointStore(store)
		}
	}
}
