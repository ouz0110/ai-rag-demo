package mcp_tool

import (
	"context"
	"encoding/json"
	"fmt"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/external/mcp"

	openai "github.com/sashabaranov/go-openai"
)

const ToolName = "call_mcp_tool"

type CallMCPTool struct {
	cfg    *conf.Config
	mcpMgr mcp.Manager
}

func NewTool(cfg *conf.Config, mcpMgr mcp.Manager) *CallMCPTool {
	return &CallMCPTool{
		cfg:    cfg,
		mcpMgr: mcpMgr,
	}
}

func (t *CallMCPTool) Definition() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        ToolName,
			Description: "通用 MCP (Model Context Protocol) 插件能力调用工具。当需要调用外部接入的 MCP 服务或扩展工具时，使用此工具传入目标服务名称 (server_name)、工具名称 (tool_name) 以及对应的输入参数 (arguments)。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"server_name": map[string]any{
						"type":        "string",
						"description": "目标 MCP 服务名称 (例如: weather_mcp)",
					},
					"tool_name": map[string]any{
						"type":        "string",
						"description": "目标 MCP 工具名称 (例如: get_weather)",
					},
					"arguments": map[string]any{
						"type":        "object",
						"description": "传递给该 MCP 工具的参数 JSON 对象",
					},
				},
				"required": []string{"server_name", "tool_name"},
			},
		},
	}
}

type CallMCPArgs struct {
	ServerName string         `json:"server_name"`
	ToolName   string         `json:"tool_name"`
	Arguments  map[string]any `json:"arguments"`
}

func (t *CallMCPTool) Run(ctx context.Context, argsJSON string) (string, error) {
	if t.mcpMgr == nil || !t.mcpMgr.IsEnabled() {
		return "", fmt.Errorf("mcp system is not enabled or initialized")
	}

	var req CallMCPArgs
	if err := json.Unmarshal([]byte(argsJSON), &req); err != nil {
		return "", fmt.Errorf("invalid json args for call_mcp_tool: %w", err)
	}

	if req.ServerName == "" || req.ToolName == "" {
		return "", fmt.Errorf("server_name and tool_name are required")
	}

	res, err := t.mcpMgr.CallTool(ctx, req.ServerName, req.ToolName, req.Arguments)
	if err != nil {
		return "", fmt.Errorf("call mcp tool failed (%s/%s): %w", req.ServerName, req.ToolName, err)
	}

	if res == nil {
		return "MCP Tool executed with empty response", nil
	}

	resBytes, err := json.Marshal(res.Content)
	if err != nil {
		return "", fmt.Errorf("marshal mcp tool result error: %w", err)
	}

	return string(resBytes), nil
}

func (t *CallMCPTool) RequiresApproval(ctx context.Context, argsJSON string) bool {
	return false
}
