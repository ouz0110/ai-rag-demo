package tool

import (
	"ai-rag-demo/internal/biz/nocli/vector"
	"ai-rag-demo/internal/conf"
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	list_file "ai-rag-demo/internal/biz/nocli/openai/tool/list_files"
	ragtool "ai-rag-demo/internal/biz/nocli/openai/tool/rag"
	readfiles "ai-rag-demo/internal/biz/nocli/openai/tool/read_files"
	terminaltool "ai-rag-demo/internal/biz/nocli/openai/tool/terminal"
)

type Tool interface {
	Definition() openai.Tool
	Run(ctx context.Context, argsJSON string) (string, error)
	RequiresApproval(argsJSON string) bool
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry(cfg *conf.Config, engines ...*vector.VectorEngine) *Registry {
	var vecEngine *vector.VectorEngine
	if len(engines) > 0 {
		vecEngine = engines[0]
	}

	return &Registry{
		tools: map[string]Tool{
			list_file.ToolName:    list_file.NewTool(cfg),
			readfiles.ToolName:    readfiles.NewTool(cfg),
			terminaltool.ToolName: terminaltool.NewTool(cfg),
			ragtool.ToolName:      ragtool.NewTool(cfg, vecEngine),
		},
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

func (r *Registry) RequiresApproval(name, argsJSON string) bool {
	t, ok := r.tools[name]
	if !ok {
		return false
	}
	return t.RequiresApproval(argsJSON)
}
