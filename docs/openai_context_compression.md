# OpenAI 上下文智能压缩与持久化方案文档

## 1. 方案概述与核心目标

本方案旨在为 `internal/biz/nocli/openai` 模块设计一套高可靠、低开销、客户端感知的上下文智能压缩与历史持久化架构。

在长对话或多轮 Tool 调用场景下，LLM 的上下文 Token 数可能会迅速达到上限。本方案重点解决以下核心问题：
1. **配置可控**：支持在配置中指定模型最大 Token 数、压缩触发比例、最大压缩次数与压缩冷却粒度。
2. **DB 历史消息与 Checkpoint 机制**：解决“数据库历史消息如何存储与高效加载”的问题，通过 Checkpoint 快照机制实现增量上下文提取，无需每次重复计算或重新总结全量历史。
3. **频次与最多次数熔断**：解决“频繁压缩无意义”的问题，通过限制最大压缩次数、最小增量消息间隔和 Rolling Summary 机制，防止上下文震荡。
4. **客户端透明与交互**：客户端仍然能获取和展示完整的历史对话全貌，并在压缩节点处呈现压缩卡片/通知。
5. **Tool-Call 协议安全**：严格保护 OpenAI API 的 Tool Call 闭包配对完整性，避免 HTTP 400 异常。

---

## 2. 深度解答：DB 历史消息处理与极简上下文获取

### 2.1 核心难题
- **问题 1**：如果将压缩后的摘要直接覆盖 DB 中的历史消息，会导致用户丢失过去真实的对话记录；如果不覆盖，每次从 DB 读取全量消息重新总结，不仅极其浪费 Token 和延迟，还会导致摘要越总结越乱。
- **问题 2**：在多 System 消息场景下（例如首部角色设定 Prompt 与中间产生的临时 RAG 内容），压缩后如何精确加载所需的数据？

### 2.2 解决方案：极简 3 段式 Checkpoint 解耦架构

中间对话过程中产生的临时 `system` 消息（如当时注入的 RAG 参考文本），其本质是过去某轮对话上下文的一部分。在触发压缩时，**它们的内容已经被融合总结到了 Checkpoint 摘要中**。

因此，发送给 OpenAI LLM 的消息队列永远严格保持以下 **3 段式极简结构**：

$$\text{LLM 运行时 Context} = \underbrace{[\text{1. 首部初始 System 消息群}]}_{\text{会话开头连续的 Base System Prompts}} + \underbrace{[\text{2. 最新 Checkpoint 摘要}]}_{\text{last\_checkpoint\_msg\_id 节点}} + \underbrace{[\text{3. 节点后的最新增量对话}]}_{\text{ID > last\_checkpoint\_msg\_id}}$$

```
DB 消息序列 (按 ID 正序):

ID: 1 | system | Agent 角色定义       ──┐
ID: 2 | system | 工作空间与工具指南    ──┴─► 【1. 首部初始 System 消息群】(固定保留)
ID: 3 | user   | 提问 1
ID: 4 | system | 中间临时注入的 RAG 内容 ──┐
ID: 5 | assist | 回答 1                  │
ID: 6 | system | Checkpoint #1 摘要      ├──► 已全部被消化提炼进 Checkpoint #2 中，彻底不用管！
ID: 7 | user   | 提问 2                  │
ID: 8 | assist | 回答 2                  ──┘
ID: 9 | system | 【Checkpoint #2 摘要】  ───► 【2. 最新 Checkpoint 摘要】(last_checkpoint_msg_id)
ID: 10| user   | 提问 3                ──┐
ID: 11| assist | 回答 3                ──┴─► 【3. 节点后的最新增量对话】(ID > 9)
```

#### 1. 数据库存储结构 (`nocli_messages` & `nocli_sessions`)
- **`nocli_messages` 增加 `msg_type` 字段**：
  - `msg_type = 0` (MsgTypeNormal)：普通对话消息（包括 Agent 原始系统提示词 Prompt、用户提问、AI 回答、Tool 结果）。普通 System 消息参与 LLM 运行时计算，但在前端聊天流中**不被渲染为卡片**。
  - `msg_type = 1` (MsgTypeCheckpoint)：上下文压缩 Checkpoint 描述消息。`MapMessageModelToStreamChunks` 识别 `msg_type == 1` 时，**精准转换为 `SET_CONTEXT_COMPRESSED` 事件卡片**给前端展示。
- **原始消息**：用户的提问、AI 的回答、Tool 调用结果按时间顺序全量追加落盘，**绝不物理删除**。
- **会话主表标记 (`nocli_sessions`)**：更新 `last_checkpoint_msg_id` 为该 Checkpoint 消息的主键 ID，更新 `compress_count = compress_count + 1`。

#### 2. 两种场景下的消息加载与数据获取机制

- **场景 A：给客户端展示/历史回放 (`GetSessionHistory`)**
  - **加载逻辑**：全量读取 DB 中的所有 Message。
  - **前端渲染**：正常渲染历史对话；当遇到 `type == "context_checkpoint"` 节点时，将其映射为 `SET_CONTEXT_COMPRESSED` 类型的 UI 分割卡片（例如：“—— 💡 以上为已压缩历史上下文 ——”），用户展开可查看摘要。

- **场景 B：给 LLM 运行时构建上下文 (`LoadHistoryForLLM`)**
  - **数据获取逻辑**：
    1. 获取【首部初始 System 消息群】（扫描会话头部连续的 `role == "system"` 消息）。
    2. 若 `last_checkpoint_msg_id > 0`：
       - 读取 `ID = last_checkpoint_msg_id` 的最新 Checkpoint 摘要消息。
       - 读取 `ID > last_checkpoint_msg_id` 的所有最新增量消息。
    3. 按 `[首部初始 System] + [最新 Checkpoint 摘要] + [增量消息]` 拼接组装。
  - **优势**：
    - **O(1) 检索效率**：无需在内存中重复分析过期的旧消息。
    - **增量滚动**：下一次压缩时，只需将“**旧 Checkpoint Summary + 旧 Checkpoint 之后的未压缩消息**”进行二次 Rolling 归纳，生成新的 Checkpoint，性能极高！

---

## 3. 频次控制与最多次数限制 (Rolling & Throttling)

### 3.1 核心难题
如果每多几条工具调用或用户回复就压缩一次，不仅浪费大量 Token 与 API 响应时间，而且频繁微调摘要会导致语义漂移。

### 3.2 解决方案：三重抑频与熔断控制

在配置中引入频次控制参数：

```yaml
source:
  openai:
    context_compress:
      enable: true
      max_context_tokens: 128000
      compress_ratio: 0.75             # 触发阈值 96000 tokens
      max_compress_count: 5            # 💡 单个会话允许的最大 LLM 压缩次数
      min_uncompressed_msgs: 6          # 💡 两次压缩之间必须积累的最小增量消息条数
      keep_recent_messages: 6          # 压缩时固定保留的最新消息数
      max_summary_tokens: 500
      timeout: 30                      # 💡 摘要生成硬超时时间 (单位：秒，默认 30 秒)
```

#### 1. 控制策略 1：最小增量消息数限制 (`min_uncompressed_msgs`)
- 在距离上次 Checkpoint 之后的未压缩消息数小于 `min_uncompressed_msgs`（如 6 条）时，即使 Token 数短暂超过阈值，也**禁止触发 LLM 摘要压缩**。
- **替代动作**：若触发压缩但未达到必要增量或 API 不可用，自动采用静默截断或延迟处理，确保主会话链路不因摘要生成失败而中断。

#### 2. 控制策略 2：最大压缩次数熔断 (`max_compress_count`)
- 一个 Session 内记录 `compress_count`。
- 当 `compress_count >= max_compress_count`（例如已压缩 5 次）：
  - **彻底停止调用 LLM 进行摘要提炼**。
  - **退化为 FIFO 滑动窗口（Sliding Window Drop）**：始终只保留 `[首部初始 System]` + `[最新的 Checkpoint 摘要]` + `[最近 N 轮对话]`，多余的中间消息直接从给 LLM 的切片中硬性剥离。

#### 3. 控制策略 3： Rolling Summary (增量滚动摘要)
- 每次压缩时，输入给总结 LLM 的 prompt 为：
  > “你是一个上下文摘要提取器。请结合【已有的历史摘要】和【最新新增的对话片段】，更新并输出一份精炼的全局对话摘要（不超过 500 字）。”
- 保证摘要长度恒定，避免摘要越长越大导致的“二次压缩死锁 (Compress Thrashing)”。

---

## 4. Tool-Call 协议安全边界算法

OpenAI API 严禁破坏 `assistant (tool_calls)` 与 `tool (tool_call_id)` 消息的成对关系。

向上寻找裁切点 (`splitIndex`) 的安全算法逻辑如下：

```
[User Msg] ── [Assistant (tool_calls: id_1)] ── [Tool (id_1)] ── [Assistant (tool_calls: id_2)] ── [Tool (id_2)] ── [User Msg]
                                                ▲
                                                │
                                    安全裁切边界 (Safe Boundary)
```

```go
func FindSafeToolBoundary(msgs []openai.ChatCompletionMessage, candidateIdx int) int {
	idx := candidateIdx
	// 如果 candidateIdx 落在 Tool 消息中间或未完成的 ToolCall Assistant 消息上，向上推移到上一条独立的 User 消息之后
	for idx > 0 && idx < len(msgs) {
		msg := msgs[idx]
		if msg.Role == openai.ChatMessageRoleTool || 
		   (msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0) {
			idx--
		} else {
			break
		}
	}
	return idx
}
```

---

## 5. 生产环境缺陷盘点与高可用应对方案 (Production Hardening)

在生产环境中落地本方案时，需特别注意以下 7 个运维与高并发层面的陷阱及应对方案：

| 生产隐患点 | 风险描述 | 生产级应对方案 (Production Solutions) |
| :--- | :--- | :--- |
| **1. 分布式并发竞态 (Race Condition)** | 用户连续快速点击或 SSE 断线重连，导致同一 Session 多个并发请求同时触发压缩，产生 DB 覆盖与重复调用 LLM。 | 基于 Redis / 本地互斥锁 (`SessionLock`) 对 SessionID 加锁，确保单 Session 的对话与压缩串行化执行。 |
| **2. 数据库事务原子性** | 插入 Checkpoint 消息成功，但更新 `nocli_sessions` 表失败，产生游离断点。 | 必须使用 `allDb.Base.InTransaction(ctx, func(txCtx) error { ... })` 开启事务，保证消息插入与 Session 主表更新强一致。 |
| **3. LLM 摘要生成超时与降级** | 生成摘要时 OpenAI 发生超时 (>8s) 或 429 限流，导致主聊天流程卡死。 | 限制摘要 LLM 请求超时为 8 秒。若超时或报错，**自动降级为静态 Hard Truncate 卡片**，保证主聊天流程 100% 可用。 |
| **4. SSE 回放与断线重连兼容** | 客户端断线重连调用 `GetSessionHistory` 时丢失“已压缩”UI 提示。 | `MapMessageModelToStreamChunks` 必须识别 `type: context_checkpoint` 并转化为 `SET_CONTEXT_COMPRESSED` 类型的 Chunk。 |
| **5. 老数据平滑兼容 (Migration)** | 线上存量 Session 的 `last_checkpoint_msg_id` 为 0。 | 当 `last_checkpoint_msg_id == 0` 时自动兼容退化为传统全量加载；SQL 迁移脚本为新列设置 `DEFAULT 0`。 |
| **6. Token 估算 CPU/内存暴发** | 每次请求都使用高精度 `tiktoken` 库逐字解析巨量文本，开销巨大。 | 采用两级估算：平时使用 `len(runes) * 0.75` 极速估算；仅在超过 85% 警戒线时触发高精度校验。 |
| **7. 可观测性与告警 (Metrics)** | 无法感知生产环境压缩触发频率、Save Token 效果及摘要失败降级率。 | 在压缩与降级点埋点结构化日志（包含 `session_id`, `compress_count`, `saved_tokens`, `duration_ms`），接入 OTel/Prometheus 监控。 |

---

## 6. 配置文件与 Proto 变更定义

### 6.1 配置定义 ([internal/conf/config.go](file:///Users/oz/code/ringkol/api-rag-demo/internal/conf/config.go))

```go
type OpenAIContextCompressConfig struct {
	Enable               bool    `json:"enable" yaml:"enable"`                                 // 是否开启
	MaxContextTokens     int     `json:"max_context_tokens" yaml:"max_context_tokens"`         // 模型最大 Context Tokens
	CompressRatio        float64 `json:"compress_ratio" yaml:"compress_ratio"`                 // 触发阈值比例 (如 0.75)
	MaxCompressCount     int     `json:"max_compress_count" yaml:"max_compress_count"`         // 💡 单会话最大压缩次数限制
	MinUncompressedMsgs  int     `json:"min_uncompressed_msgs" yaml:"min_uncompressed_msgs"`   // 💡 两次压缩间最小增量消息条数
	KeepRecentMessages   int     `json:"keep_recent_messages" yaml:"keep_recent_messages"`     // 压缩时保留的最新消息数
	MaxSummaryTokens     int     `json:"max_summary_tokens" yaml:"max_summary_tokens"`         // 摘要最大 Token
}
```

### 6.2 Proto 变更 ([api/nocli/v1/chat.proto](file:///Users/oz/code/ringkol/api-rag-demo/api/nocli/v1/chat.proto))

```protobuf
enum StreamEventType {
  SET_UNSPECIFIED = 0;
  SET_TEXT_DELTA = 1;
  SET_REASONING = 2;
  SET_TOOL_START = 3;
  SET_TOOL_RESULT = 4;
  SET_INTERRUPT = 5;
  SET_DONE = 6;
  SET_ERROR = 7;
  SET_CONTEXT_COMPRESSED = 8; // 上下文压缩触发事件
}

message CompressInfo {
  int32 original_tokens = 1;
  int32 compressed_tokens = 2;
  int32 compressed_msg_count = 3;
  int32 compress_count = 4;      // 当前已压缩次数
  string summary_preview = 5;    // 摘要预览
}

message StreamChunk {
  // ... 现有字段 ...
  CompressInfo compress_info = 11; // 压缩详情
}
```

---

## 7. 代码落地修改清单

| 修改文件 | 责任说明 |
| :--- | :--- |
| [internal/conf/config.go](file:///Users/oz/code/ringkol/api-rag-demo/internal/conf/config.go) | 扩展 `OpenAIContextCompressConfig` 结构体及 `OpenAI` 嵌套字段 |
| `configs/config.yaml` | 增加上下文压缩与频次熔断参数配置 |
| [api/nocli/v1/chat.proto](file:///Users/oz/code/ringkol/api-rag-demo/api/nocli/v1/chat.proto) | 新增 `SET_CONTEXT_COMPRESSED` 事件与 `CompressInfo` 结构 |
| `sqls/20260729_add_context_compress_to_sessions.sql` *(新建)* | 给 `nocli_sessions` 表添加 `compress_count`, `last_checkpoint_msg_id`, `last_compressed_at` |
| `internal/biz/nocli/openai/compressor/compressor.go` *(新建)* | 实现 Checkpoint 生成、Tool-Call 安全边界判断、Token 估算与 8 秒降级逻辑 |
| [internal/data/base/nocli_session.go](file:///Users/oz/code/ringkol/api-rag-demo/internal/data/base/nocli_session.go) | 适配 `NocliSessionModel` 新增字段，提供 `UpdateCompressStatus` 原子更新方法 |
| [internal/biz/nocli/session/history.go](file:///Users/oz/code/ringkol/api-rag-demo/internal/biz/nocli/session/history.go) | 实现 `LoadHistoryForLLM` 逻辑，支持基于 Checkpoint 快照只读增量数据 |
| [internal/biz/nocli/session/manager.go](file:///Users/oz/code/ringkol/api-rag-demo/internal/biz/nocli/session/manager.go) | 在 `FinalizeSessionTurn` 中开启 DB 事务落盘 Checkpoint 节点并更新 Session |
| [internal/biz/nocli/openai/agent/base/run.go](file:///Users/oz/code/ringkol/api-rag-demo/internal/biz/nocli/openai/agent/base/run.go) | 在 Agent 执行主循环中集成压缩器与 SSE `SET_CONTEXT_COMPRESSED` 事件推送 |

---

## 8. 方案总结

本方案通过 **“首部初始 System 消息 + 最新 Checkpoint 摘要 + 增量消息”** 的 3 段式解耦架构，彻底解决了多 System 场景下的历史提取难题；通过 **最大压缩次数限制 + 最小消息间隔 + Rolling Summary** 解决了频繁压缩的无效开销问题；通过 **Tool-Call Safe Boundary 与 生产环境高可用应对（分布式锁、DB事务、超时降级）** 保证了生产环境下的绝对稳定可靠。
