# AI RAG Demo API (ai-rag-demo)

[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Framework](https://img.shields.io/badge/Framework-Kratos%20v2-blue)](https://go-kratos.dev/)
[![Vector Store](https://img.shields.io/badge/VectorDB-Milvus%20v2-00A4E4)](https://milvus.io/)
[![Protocol](https://img.shields.io/badge/MCP-Enabled-orange)](https://modelcontextprotocol.io/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## 📌 项目概述

**AI RAG Demo API** 是一个基于 **Go 语言 (Kratos v2 框架)** 构建的企业级、高性能、云原生 AI Agent 与 RAG（检索增强生成）知识库问答后端系统。

本项目采用了现代化的微服务架构与 Clean Architecture（干净架构）设计，提供了从**多格式文档解析**、**高级 RAG 向量引擎 (Advanced RAG)**、**多 Agent 协同编排**、**MCP (Model Context Protocol) 扩展**、**Skills 动态加载**，到**上下文长文本压缩**、**流式打字机响应 (Stream SSE)** 以及**多租户计费与账户系统 (Accounts & Billing)** 的全栈能力。

---

## 🔥 核心功能特性

### 1. 高级 RAG (Retrieval-Augmented Generation) 向量引擎
- **多格式文档解析 (`vector/parser`)**: 内置 Markdown、JSON、CSV、TXT 等格式文档提取器，自动提取清洗文本。
- **灵活切块策略 (`vector/chunker`)**:
  - **静态固定切片 (`Static Chunker`)**: 基于固定窗口和 Overlap 滑动切分。
  - **语义切分 (`Semantic Chunker`)**: 基于文本语义变化点与相似度阀值动态切分。
  - **父子层级切片 (`Hierarchical Chunker`)**: 保留大文本块上下文（Parent），检索小精细块（Child），解决上下文断裂问题。
- **混合检索召回 (`vector/retriever`)**: 支持基于 Milvus 的 Dense Vector 语义检索与多路 Hybrid 混合检索。
- **二阶段精排 (`vector/rerank`)**: 组合基础召回与 Rerank 引擎（内置 LLM Reranker 与百度千帆 Reranker），确保检索精准度。
- **知识库隔离与构建流水线 (`kb_biz.go`)**: 支持多租户隔离、系统级公共知识库与用户自定义知识库的上传、管理与向量索引化构建。

### 2. 智能 Agent 编排与插件生态
- **多 Agent 协同注册中心 (`openai/agent`)**:
  - **Main Agent**: 智能意图路由与调度中心。
  - **RAG Agent**: 专门负责知识库精准问答与依据归因。
  - **File Analyzer Agent**: 针对上传文件的深度结构化分析与摘要提取。
  - **AgentTool 级联抽象 (`agent_tool.go`)**: 将 Agent 统一封装为标准 Tool，实现 Agent 间的递归调用与任务委派。
- **MCP (Model Context Protocol) 标准集成**: 基于 `mark3labs/mcp-go`，无缝接入第三方 MCP 工具与外部上下文资源。
- **Skills 动态扩展管理 (`internal/pkg/skill`)**: 支持动态扫描与运行自定义本地/远程技能，让 Agent 具备按需调用工具的能力。
- **内置丰富工具箱 (`openai/tool`)**: 包含终端命令执行 (`terminal`)、文件读写 (`read_files`/`list_files`)、RAG 检索 (`rag`)、Skill 加载 (`load_skill`) 及 MCP 动态工具封装 (`mcp_tool`)。

### 3. Human-in-the-Loop (HITL) 人工介入与安全审批控制
- **高危工具授权策略 (`session/approved_tools.go`)**: 支持 `AS_ALWAYS` (永久授权)、`AS_SESSION_TOOL` (单会话授权) 与拒绝机制。当 Agent 尝试调用敏感命令（如 `terminal` 命令或写操作）时自动阻断并触发 `SS_INTERRUPTED` 状态。
- **中断与断点恢复 (`stream_chat.go`)**: 任务挂起后，支持用户选择同意（Approve）或拒绝（Reject）并通过 `StreamResume` / `ResumeCompletion` 实现秒级断点恢复执行。
- **新 Prompt 智能解理**: 若用户在审批等待期间发送全新 Prompt，系统能够智能清理/撤销原挂起中断并顺畅收尾消息链。

### 4. SubAgent Checkpoint 快照与秒级续跑引擎 (`checkpoint`)
- **轻量级快照存储 (`checkpoint/checkpoint.go`)**: 子 Agent 触发中断时，实时捕获并持久化局部上下文 (`SubMessages`)、Pending Tool Call、控制选项 (`AgentToolOptions`)、知识库/技能开关及已授权白名单。
- **秒级精确唤醒 (`trySubAgentCheckpointResume`)**: 恢复时优先激活对应子 Agent 的 Checkpoint 快照秒级续跑，规避父 Agent 全量重新调度，降低响应延迟与 Token 成本。

### 5. 上下文控制、主动打断与性能优化
- **长文本上下文压缩器 (`openai/compressor`)**: 自动评估多轮对话 Token 消耗，使用摘要压缩策略优化上下文窗口，降低 LLM 调用成本并防超限。
- **流式增量打字机 (`stream_chat.go`)**: 支持 SSE (Server-Sent Events) 与 HTTP/gRPC 流式响应，提供毫秒级首字延迟与流畅交互。
- **主动任务打断与接续生成 (`chat.go`)**:
  - **任务中断 (`StopSession`)**: 基于 `activeCancels` 句柄提供优雅的会话级打断控制，安全将任务切换至 `SS_PAUSED` 状态。
  - **接续生成 (`IsContinue`)**: 自动从中断或未完成的 Assistant 输出末尾接续往下生成，保障长文本输出的连续性。
- **脱钩落盘安全保护 (`context.WithoutCancel`)**: 即使客户端中途切断 HTTP/SSE 连接，后台仍借助独立超时上下文保障对话记录与工具耗时 100% 完整安全落盘。
- **细粒度耗时归因与配额记账 (`common/usage.go`)**: 实时记录各工具耗时 (`ToolDurations`)，并实现 Prompt/Completion/Embedding Token 的统一计量与并发扣费。

### 6. 商业化与微服务基础设施
- **账户与权限管理 (`base/accounts.go`)**: 支持用户注册、登录、JWT 认证、多租户隔离及 OpenID 绑定。
- **Token 消耗与计费 (`base/billing.go`)**: 实时统计 LLM / RAG / Embedding 的 Token 消耗与额度扣减。
- **微服务治理**: 支持 Nacos 服务注册与发现、配置动态加载、Google Wire 依赖注入。
- **全栈可观测性**: 集成 OpenTelemetry Tracing/Metrics、Prometheus 监控指标导出与 Zap 异步日志切割。

---

## 🛠️ 技术栈 (Tech Stack)

| 领域 / 组件 | 选型技术 / 依赖包 | 说明 |
| :--- | :--- | :--- |
| **核心语言** | Go 1.26+ | 高并发、强类型 |
| **微服务框架** | [Go-Kratos v2](https://github.com/go-kratos/kratos) | 微服务通信、HTTP/gRPC 双协议支持 |
| **依赖注入** | [Google Wire](https://github.com/google/wire) | 编译期依赖注入，清晰解耦 |
| **大语言模型** | OpenAI API (`go-openai`) / 千帆 Reranker | 兼容主流 LLM 与精排模型 |
| **向量数据库** | [Milvus](https://milvus.io/) (`milvus-sdk-go/v2`) | 高性能海量向量存储与检索 |
| **关系型数据库**| MySQL 8.0 / [GORM v2](https://gorm.io/) | ORM 映射、事务控制与数据持久化 |
| **缓存与锁** | Redis (`go-redis/v9`) | 分布式缓存与并发控制 |
| **服务发现/配置**| Nacos (`nacos-sdk-go`) | 微服务注册中心与动态配置 |
| **标准协议** | [MCP Protocol](https://modelcontextprotocol.io/) (`mcp-go`) | 模型上下文协议适配 |
| **可观测性** | OpenTelemetry, Prometheus, Zap | 分布式链路追踪、Metrics 与日志收集 |

---

## 🏗️ 项目架构与目录设计 (`internal/biz` 核心)

项目遵循领域驱动设计 (DDD) 与 Kratos 标准目录划分：

```text
api/                  # Proto 协议定义层 (HTTP/gRPC 路由、错误码、DTO)
cmd/server/           # 程序入口 (main.go, wire 注入点)
configs/              # 配置文件 (YAML / Nacos)
internal/
├── biz/              # ⭐️ 核心业务逻辑层 (领域编排，禁跨包调用)
│   ├── base/         # 基础业务：账号管理 (accounts.go)、计费管控 (billing.go)
│   ├── common/       # 共享业务逻辑与配额 Usage 统一记录 (usage.go)
│   └── nocli/        # 核心 AI 与 RAG 领域引擎
│       ├── checkpoint/# SubAgent 中断快照存储 (checkpoint.go, memory_store.go)
│       ├── session/  # 会话历史持久化、已授权工具白名单 (approved_tools.go, manager.go)
│       ├── vector/   # Advanced RAG 向量引擎 (parser, chunker, embedder, store, rerank, retriever)
│       ├── openai/   # 多 Agent 协同 (agent)、工具箱 (tool)、ChatModel 与长文本压缩 (compressor)
│       ├── kb_biz.go # 知识库 CRUD 与文件向量构建流水线
│       ├── chat.go   # 问答编排、主动中断控制 (StopSession)
│       └── stream_chat.go # 打字机 SSE/gRPC 流式响应、HITL 审批恢复与断点接续
├── data/             # 数据持久化层 (GORM DB CRUD, Milvus Adapter)
├── cache/            # 缓存层 (Redis & 分布式锁)
├── external/         # 外部服务集成 (MCP Manager, RPC Proxy)
├── pkg/              # 公共组件 (Skills 扫描管理器、Log、Utils)
└── server/           # HTTP/gRPC 服务端初始化
```

---

## ✨ 架构设计亮点与规范

1. **分层清晰与领域解耦**:
   - 严格区分 `api` (定义)、`service` (协议转换)、`biz` (业务编排) 与 `data` (底层存储)。
   - 遵守 **跨 Biz 调用禁令**，共享逻辑统一下沉至 `biz/common`，确保业务模块间高内聚、低耦合。
2. **高级 RAG 策略组合**:
   - 不止于简单的 Vector Search，引入**语义切块 + 父子层级切分 + 混合召回 + 重排序 (Rerank)** 的完整 Advanced RAG 链条，大幅提升特定领域问答质量。
3. **Human-in-the-Loop (HITL) 与 SubAgent Checkpoint**:
   - 对敏感高危操作提供可可控的人工确认机制 (`SS_INTERRUPTED`)；
   - 结合子 Agent Checkpoint 快照存储，恢复授权时可实现秒级精准断点续跑，跳过全量重新调度。
4. **扩展性极强 (MCP & Skills)**:
   - 引入 Anthropic 主导的 **MCP (Model Context Protocol)** 标准，支持灵活连接上下文数据源；
   - 自研 **Skills 管理器**，支持本地动态扫描与按需加载能力插件。
5. **任务优雅控制与脱钩落盘安全**:
   - 借助 `activeCancels` 实现线程安全的会话级主动任务打断 (`StopSession`)；
   - 引入独立超时上下文 (`context.WithoutCancel`)，确保前端连接断开时对话记录与工具耗时 (`ToolDurations`) 依然 100% 完整落盘。
6. **安全与鲁棒性防护**:
   - 强制使用统一安全协程 `common.RunInGoroutine`，防范后台 Panic 导致服务宕机。
   - 所有 HTTP/gRPC 请求接口均支持强类型 `protoc-gen-validate` 校验与统一的身份认证上下文传递 (`UserFromContext`)。

---

## 🚀 启动与依赖说明

为保证系统的完整运行，请依次准备并启动**基础设施服务**、**本地 Embedding/Reranker 模型服务**，随后启动后端微服务。

---

### 🛠️ 1. 基础设施依赖服务

在启动应用前，请确保以下基础组件已正常运行：

| 服务名称 | 默认端口 | 依赖说明 |
| :--- | :--- | :--- |
| **MySQL** | `3306` | 用于持久化保存账号、计费、知识库元数据与对话 Session |
| **Redis** | `6379` | 用于分布式缓存、会话锁与高频读写 |
| **Milvus** | `19530` / `9091` | 高性能向量数据库，用于向量检索与存储 |
| **Nacos** | `8848` / `80` | (可选) 配置中心与服务注册发现中心 |

---

### 🤖 2. 本地 AI 模型服务 (Embedding & Reranker)

系统支持连接第三方 OpenAI API 或基于 `llama.cpp` (`llama serve`) 本地部署轻量高性能的 Embedding 与 Reranker 模型服务。

#### 转发与启动命令：

* **启动 Reranker 重排序服务 (端口 8090)**
  使用 `gpustack/bge-reranker-v2-m3-GGUF` 提供二阶段文本语义精排：
  ```bash
  llama serve -hf gpustack/bge-reranker-v2-m3-GGUF:Q8_0 \
    --port 8090 \
    --reranking \
    -b 4096 \
    -c 8192 \
    -ub 4096
  ```

* **启动 Embedding 向量化服务 (端口 8091)**
  使用 `Qwen/Qwen3-Embedding-0.6B-GGUF` 提供高效稠密向量提取：
  ```bash
  llama serve -hf Qwen/Qwen3-Embedding-0.6B-GGUF:Q8_0 \
    --port 8091 \
    --embeddings
  ```

---

### ⚙️ 3. 配置文件设置

在配置文件或环境参数中关联上述基础设施与 AI 模型端点（以 `configs/config.yaml` 为例）：

```yaml
source:
  # 数据库与缓存
  database:
    driver: mysql
    source: root:password@tcp(127.0.0.1:3306)/ai_rag_demo?parseTime=True&loc=Local
  redis:
    addr: 127.0.0.1:6379
  milvus:
    address: 127.0.0.1:19530

  # 本地/远程模型服务配置
  openai:
    api_key: "your-openai-api-key"
    # 本地 Embedding 服务端点 (llama serve @ 8091)
    embedding_base_url: "http://127.0.0.1:8091/v1"
    # 本地 Reranker 服务端点 (llama serve @ 8090)
    rerank_base_url: "http://127.0.0.1:8090/v1"
```

---

### 📦 4. 项目编译与服务启动

1. **获取源码与拉取依赖**
   ```bash
   git clone <repository_url>
   cd api-rag-demo
   go mod download
   ```

2. **代码生成 (Proto API & Wire 注入)**
   ```bash
   # 初始化编译环境
   make init

   # 生成 Proto HTTP/gRPC 代码与结构体校验代码
   make api

   # 执行 Google Wire 编译期依赖注入生成
   make wire

   # 构建全部依赖(本地运行)
   docker-compose up -d
   ```

3. **启动应用服务**
   ```bash
   # 复制配置文件，修改相关配置
   cp configs/config.local.yaml.example configs/config.local.yaml

   # 启动后端服务 (本地运行)
   go run ./cmd/server --conf configs/config.local.yaml
   ```

---

## 📝 许可证

[MIT License](LICENSE)