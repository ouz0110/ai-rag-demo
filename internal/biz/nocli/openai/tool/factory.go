package tool

import (
	"ai-rag-demo/internal/conf"
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	list_file "ai-rag-demo/internal/biz/nocli/openai/tool/list_files"
	readfiles "ai-rag-demo/internal/biz/nocli/openai/tool/read_files"
)

type Tool interface {
	Definition() openai.Tool
	Run(ctx context.Context, argsJSON string) (string, error)
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry(cfg *conf.Config) *Registry {
	return &Registry{
		tools: map[string]Tool{
			list_file.ToolName:  list_file.NewTool(cfg),
			readfiles.ToolName: readfiles.NewTool(cfg),
		},
	}
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
