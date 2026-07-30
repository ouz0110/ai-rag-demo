package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/log"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

type Client struct {
	cfg        conf.MCPServerConfig
	clientInfo mcp.Implementation

	mu         sync.RWMutex
	state      ConnState
	sseClient  *mcpclient.Client
	tools      []mcp.Tool
	cancelFunc context.CancelFunc
}

func NewClient(cfg conf.MCPServerConfig, clientInfo conf.MCPClientInfo) *Client {
	return &Client{
		cfg:   cfg,
		state: StateDisconnected,
		clientInfo: mcp.Implementation{
			Name:    clientInfo.Name,
			Version: clientInfo.Version,
		},
	}
}

// Connect 初始化并建立 SSE 长连接与 Handshake
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.state == StateConnected || c.state == StateConnecting {
		c.mu.Unlock()
		return nil
	}
	c.state = StateConnecting
	c.mu.Unlock()

	// 1. 构建 SSE Client Options
	var opts []transport.ClientOption
	if len(c.cfg.Headers) > 0 {
		opts = append(opts, mcpclient.WithHeaders(c.cfg.Headers))
	}

	sseClient, err := mcpclient.NewSSEMCPClient(c.cfg.URL, opts...)
	if err != nil {
		c.setDisconnected()
		return fmt.Errorf("create sse mcp client failed for %s: %w", c.cfg.Name, err)
	}

	// 2. 建立后台 SSE 接收 Context
	connCtx, cancel := context.WithCancel(context.Background())
	if err := sseClient.Start(connCtx); err != nil {
		cancel()
		c.setDisconnected()
		return fmt.Errorf("start sse connection failed for %s: %w", c.cfg.Name, err)
	}

	// 3. 执行 MCP Initialize 握手
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = c.clientInfo

	timeout := c.cfg.Timeout.Duration
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	initCtx, initCancel := context.WithTimeout(ctx, timeout)
	defer initCancel()

	_, err = sseClient.Initialize(initCtx, initReq)
	if err != nil {
		_ = sseClient.Close()
		cancel()
		c.setDisconnected()
		return fmt.Errorf("initialize mcp handshake failed for %s: %w", c.cfg.Name, err)
	}

	c.mu.Lock()
	c.sseClient = sseClient
	c.cancelFunc = cancel
	c.state = StateConnected
	c.mu.Unlock()

	log.Infow(ctx, "mcp sse client connected successfully", "server", c.cfg.Name)

	// 4. 尝试获取并缓存 Tools 列表
	c.RefreshTools(ctx)

	return nil
}

func (c *Client) RefreshTools(ctx context.Context) {
	c.mu.RLock()
	client := c.sseClient
	state := c.state
	c.mu.RUnlock()

	if state != StateConnected || client == nil {
		return
	}

	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.ListTools(listCtx, mcp.ListToolsRequest{})
	if err != nil {
		log.Errorw(ctx, "failed to list tools from mcp server", "server", c.cfg.Name, "error", err)
		return
	}

	c.mu.Lock()
	c.tools = resp.Tools
	c.mu.Unlock()
}

func (c *Client) CallTool(ctx context.Context, toolName string, arguments map[string]any) (*mcp.CallToolResult, error) {
	c.mu.RLock()
	client := c.sseClient
	state := c.state
	c.mu.RUnlock()

	if state != StateConnected || client == nil {
		return nil, fmt.Errorf("mcp server %s is not connected (state: %s)", c.cfg.Name, state)
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = arguments

	return client.CallTool(ctx, req)
}

func (c *Client) GetTools() []mcp.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tools
}

func (c *Client) State() ConnState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Client) setDisconnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = StateDisconnected
	c.sseClient = nil
	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = StateClosed
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	if c.sseClient != nil {
		return c.sseClient.Close()
	}
	return nil
}
