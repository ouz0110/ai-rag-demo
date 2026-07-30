package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/log"

	"github.com/mark3labs/mcp-go/mcp"
)

type DefaultManager struct {
	enabled bool
	cfg     *conf.MCPConfig
	clients map[string]*Client
	mu      sync.RWMutex
	stopCh  chan struct{}
}

func NewManager(cfg *conf.Config) (Manager, error) {
	mgr := &DefaultManager{
		clients: make(map[string]*Client),
		stopCh:  make(chan struct{}),
	}

	if cfg == nil || cfg.Source.MCP == nil || !cfg.Source.MCP.Enable {
		mgr.enabled = false
		log.Infow(context.Background(), "mcp system is disabled globally")
		return mgr, nil
	}

	mcpCfg := cfg.Source.MCP
	mgr.cfg = mcpCfg
	mgr.enabled = true

	// 初始化客户端列表
	for _, sCfg := range mcpCfg.Servers {
		if !sCfg.Enabled {
			continue
		}
		client := NewClient(sCfg, mcpCfg.ClientInfo)
		mgr.clients[sCfg.Name] = client
	}

	// 启动阶段尝试首次连通
	mgr.initConnections()

	// 遵循 AGENTS.md 规范：禁止原生 go func()，必须使用 common.RunInGoroutine
	common.RunInGoroutine(context.Background(), func(ctx context.Context) {
		mgr.startHealthCheckAndReconnect(ctx)
	})

	return mgr, nil
}

func (m *DefaultManager) IsEnabled() bool {
	return m.enabled
}

func (m *DefaultManager) initConnections() {
	timeout := 30 * time.Second
	if m.cfg != nil && m.cfg.DefaultTimeout.Duration > 0 {
		timeout = m.cfg.DefaultTimeout.Duration
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, client := range m.clients {
		c := client
		if err := c.Connect(ctx); err != nil {
			log.Warnw(ctx, "initial connect to mcp server failed, will retry in background",
				"server", c.cfg.Name, "error", err)
		}
	}
}

// startHealthCheckAndReconnect 定时心跳与自动重连机制
func (m *DefaultManager) startHealthCheckAndReconnect(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.RLock()
			clients := make([]*Client, 0, len(m.clients))
			for _, c := range m.clients {
				clients = append(clients, c)
			}
			m.mu.RUnlock()

			for _, c := range clients {
				if c.State() != StateConnected {
					log.Infow(ctx, "attempting to reconnect mcp server", "server", c.cfg.Name)
					reconnCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
					if err := c.Connect(reconnCtx); err != nil {
						log.Warnw(ctx, "reconnect mcp server failed", "server", c.cfg.Name, "error", err)
					}
					cancel()
				}
			}
		}
	}
}

func (m *DefaultManager) CallTool(ctx context.Context, serverName, toolName string, arguments map[string]any) (*mcp.CallToolResult, error) {
	if !m.enabled {
		return nil, fmt.Errorf("mcp system is disabled globally")
	}

	client, ok := m.GetClient(serverName)
	if !ok {
		return nil, fmt.Errorf("mcp server %s not found or not enabled", serverName)
	}

	return client.CallTool(ctx, toolName, arguments)
}

func (m *DefaultManager) ListAllTools(ctx context.Context) ([]ServerTool, error) {
	if !m.enabled {
		return nil, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []ServerTool
	for serverName, client := range m.clients {
		if client.State() == StateConnected {
			tools := client.GetTools()
			for _, t := range tools {
				result = append(result, ServerTool{
					ServerName: serverName,
					Tool:       t,
				})
			}
		}
	}
	return result, nil
}

func (m *DefaultManager) GetClient(serverName string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.clients[serverName]
	return client, ok
}

func (m *DefaultManager) Close() error {
	if !m.enabled {
		return nil
	}

	close(m.stopCh)
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		_ = client.Close()
	}
	return nil
}
