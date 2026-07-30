# MCP (Model Context Protocol) SSE 连接与 Agent 独立物理工具集成方案

## 一、概述

本方案基于 Go 语言开源包 `github.com/mark3labs/mcp-go`，针对大模型 Agent 架构中的工具插件扩展（Model Context Protocol），设计了一套纯 **SSE (Server-Sent Events)** 传输协议驱动、支持高可用重连、状态监控以及遵循“单一职责”的独立物理工具集成方案。

方案遵循 `AGENTS.md` 规范与单一职责原则：**Tool 层只负责物理调用转发（`call_mcp_tool`），Agent 层面进行工具注册与 System Prompt 增强，实现了完全解耦与可控加载。**

---

## 二、架构设计与工具调用全链路

```
+-----------------------------------------------------------------------------------+
|                                  Client Request                                   |
|       (pb.CompletionRequest / pb.ResumeRequest -> EnableMCP: true/false)          |
+-----------------------------------------+-----------------------------------------+
                                          |
                                          v
+-----------------------------------------+-----------------------------------------+
|                  resolveEnableFlags(cfg, reqRAG, reqSkill, reqMCP)                |
|       (全局配置 cfg.Source.MCP.Enable && 请求参数 req.EnableMCP => finalMCP)        |
+-----------------------------------------+-----------------------------------------+
                                          |
                                          v
+-----------------------------------------+-----------------------------------------+
|                          ChatBiz / ParentContext                                  |
|            (若 finalMCP == false, 则跳过 MCP Prompt 增强与感知)                     |
+-----------------------------------------+-----------------------------------------+
                                          | (若 finalMCP == true)
                                          v
+-----------------------------------------+-----------------------------------------+
|                            BaseAgent / MainAgent                                  |
| 1. System Prompt: 自动注入已连通 MCP 服务及工具清单 (引导 LLM 使用 call_mcp_tool)    |
| 2. Physical Tool: 绑定物理工具 call_mcp_tool                                      |
+-----------------------------------------+-----------------------------------------+
                                          |
                                          | (LLM 决策发起 Tool Call: call_mcp_tool)
                                          v
+-----------------------------------------+-----------------------------------------+
|                   mcptool.CallMCPTool (tool/mcp_tool/tool.go)                     |
|           (解析传入的 server_name, tool_name 与 arguments JSON 对象)               |
+-----------------------------------------+-----------------------------------------+
                                          |
                                          v
+-----------------------------------------+-----------------------------------------+
|                    internal/external/mcp/Manager                                  |
|                        (MCP Client 连接池总控)                                    |
+----------------------+------------------------------------+-----------------------+
                       |                                    |
                       v                                    v
+----------------------+-------------------+ +--------------+-----------------------+
|            MCP Client A                  | |            MCP Client B              |
|   (internal/external/mcp/client)         | |   (internal/external/mcp/client)     |
+----------------------+-------------------+ +--------------+-----------------------+
                       | (SSE Protocol)                     | (SSE Protocol)
                       v                                    v
+----------------------+-------------------+ +--------------+-----------------------+
|        Remote MCP Server A               | |        Remote MCP Server B           |
|    (e.g., Weather MCP Service)           | |     (e.g., Database Service)         |
+------------------------------------------+ +--------------------------------------+
```

---

## 三、关键特性设计

### 1. 配置驱动与可控加载 (Controllable Loading)
借鉴项目中 `finalSkill` / `finalRAG` 的设计机制，MCP 能力的使能采取“两级控制”：
- **第一级（全局总控）**：配置文件 `configs/config.local.yaml` 中的 `source.mcp.enable` 控制系统启动时是否初始化 MCP 管理器和建立 SSE 连接。
- **第二级（请求级细粒度控制）**：每个 API 请求 (`CompletionRequest` / `ResumeRequest`) 中的 `enable_mcp` 参数。
- **解析逻辑**：
  $$\text{finalMCP} = \text{cfg.Source.MCP.Enable} \land \text{req.EnableMCP}$$
  只有当全局配置开启且用户请求显式启用时，`finalMCP` 才为 `true`。若 `finalMCP` 为 `false`，Agent 在当前对话 Turn 中将不会注入 MCP 提示词说明。

### 2. 独立物理工具 (Standalone `call_mcp_tool`)
- **单一职责**：工具只负责接收结构化的参数 `server_name`、`tool_name` 和 `arguments`，并将调用转发给 `mcpManager.CallTool(...)`。
- **参数格式定义**：
  ```json
  {
    "server_name": "weather_mcp",
    "tool_name": "get_weather",
    "arguments": { "city": "Beijing" }
  }
  ```

### 3. Agent 显式注册与 System Prompt 引导
- 在 `MainAgent` 构造阶段，将 `call_mcp_tool` 注册进 Agent 可调用的物理工具字典中（与 `load_skill` / `read_files` 并列）。
- 在 `EnhanceRuntimeMessages` 中，根据 `finalMCP` 的值，将当前所有连通的 MCP 服务以及它们暴露的工具名称与描述拼接到 System Prompt 中（类似 Skill 的 Level 1 Prompt），明确引导 LLM 调用 `call_mcp_tool`。

### 4. SSE 长连接与退避重连 (Exponential Backoff Reconnect)
- 单个 MCP 服务连接采用 `mcp-go/client.NewSSEMCPClient` 实例化。
- 在后台心跳检测与重连逻辑中，使用项目标准 `common.RunInGoroutine` 进行防护，严禁直接使用原生 `go func()`，保证异常 panic 被安全捕获并记录日志。
- 当远端 SSE 服务重启或网络短时间中断时，自动尝试重连握手 (`Initialize`)，并在连通后自动更新 Tool 列表缓存。

---

## 四、配置设计说明

配置文件格式（YAML）如下：

```yaml
source:
  mcp:
    enable: true                            # 是否开启 MCP 工具扩展系统全局总开关
    client_info:
      name: "ai-rag-demo-agent"
      version: "1.0.0"
    default_timeout: 30s
    servers:
      - name: "weather_mcp"
        description: "天气查询 MCP 服务"
        url: "http://localhost:8080/sse"
        enabled: true
        headers:
          Authorization: "Bearer token-xxxx"
        timeout: 10s
        max_retries: 5
        retry_interval: 2s
```

---

## 五、核心代码模块分布

1. `internal/conf/config.go`:
   - 增加 `MCPConfig`、`MCPServerConfig` 以及 `MCPClientInfo` 配置映射结构体。
2. `internal/external/mcp/`:
   - `status.go`: 核心接口 `Manager`、状态定义 `ConnState`、数据传输模型。
   - `client.go`: 单节点 SSE 长连接封装，负责握手 `Initialize`、工具列表 `ListTools` 与调用 `CallTool`。
   - `manager.go`: MCP 连接池，管理多节点生命周期、心跳与重连。
3. `internal/biz/nocli/openai/tool/mcp_tool/tool.go`:
   - 独立物理工具 `call_mcp_tool` 的实现。
4. `internal/biz/nocli/openai/agent/main_agent.go`:
   - 显式绑定 `call_mcp_tool` 物理工具。
5. `internal/biz/nocli/openai/agent/base/base.go`:
   - 包含 `BuildMCPPromptFromContext`，支持动态拼接连通的 MCP 服务与工具列表说明。
6. `api/nocli/v1/chat.proto`:
   - 在 `CompletionRequest` 与 `ResumeRequest` 中新增 `enable_mcp` 字段。
7. `internal/biz/nocli/chat.go` & `stream_chat.go`:
   - 在 `resolveEnableFlags` 中解析出 `finalMCP` 并下发至 `ParentContext`。

---

## 六、验证与测试

1. 修改配置 `source.mcp.enable: true`，启动 server，查看控制台日志输出 `mcp sse client connected successfully`。
2. 在请求中传入 `"enable_mcp": true`，模型提示词中自动拼接当前可用的 MCP 服务与工具列表。
3. 大模型分析需求后，主动发起对 `call_mcp_tool` 的调用并传入 `server_name` 和 `tool_name`。
