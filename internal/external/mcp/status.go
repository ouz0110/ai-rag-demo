package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

type ConnState string

const (
	StateDisconnected ConnState = "DISCONNECTED"
	StateConnecting   ConnState = "CONNECTING"
	StateConnected    ConnState = "CONNECTED"
	StateClosed       ConnState = "CLOSED"
)

// ServerTool 包含 Server 名称及绑定的 Tool 结构
type ServerTool struct {
	ServerName string
	Tool       mcp.Tool
}

// Manager MCP 客户端连接池管理总控接口
type Manager interface {
	// CallTool 调用指定 Server 的指定 Tool
	CallTool(ctx context.Context, serverName, toolName string, arguments map[string]any) (*mcp.CallToolResult, error)
	// ListAllTools 聚合获取所有已建立 SSE 连通的 MCP Server 暴露的 Tool 列表
	ListAllTools(ctx context.Context) ([]ServerTool, error)
	// GetClient 根据名称获取底层 Client
	GetClient(serverName string) (*Client, bool)
	// IsEnabled 当前全局配置下 MCP 是否启用
	IsEnabled() bool
	// Close 优雅关闭所有 SSE 连接
	Close() error
}
