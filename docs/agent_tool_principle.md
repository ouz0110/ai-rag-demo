# AgentTool 工作原理总结

> 学习 & 面试用：掌握多智能体编排核心组件的设计思想与工作流程。

---

## 一、核心定位（一句话）

**`AgentTool` 将内部定义的 `IAgent` 适配成 OpenAI 的 `Function/Tool`，让主 Agent（MainAgent）能够像调用普通工具一样，动态调度子 Agent 完成委派任务。**

> 这是实现 **Orchestrator（编排模式）多智能体系统**的核心组件。

---

## 二、关键结构体

| 结构体 | 作用 |
|--------|------|
| `AgentToolArgs` | Tool 调用入参，只有一个 `query` 字段，即交给子 Agent 的任务描述 |
| `AgentToolOptions` | 控制父子 Agent 交互的行为开关（3 个 bool） |
| `AgentTool` | 持有 `targetAgent`、`chatModel`、`opts`，实现 OpenAI Tool 协议 |

### `AgentToolOptions` 三个开关（面试高频点）

- **`PassFullContextToSubAgent`**（默认 `false`）：父 → 子，是否把父 Agent 的完整历史消息透传给子 Agent。
- **`ReturnFullContextToParent`**（默认 `false`）：子 → 父，子 Agent 执行完成后，将自身的完整多轮消息历史通过回调追加回父 Agent 的消息栈，供父 Agent 后续轮次使用。
- **`StreamSubAgentExecution`**（默认 `true`）：子 Agent 执行时是否把中间推理/思考过程实时流式推送给用户。

---

## 三、核心工作流（`Run` 方法逐行拆解）

```
MainAgent 决定调用 agent_tool
        ↓
Run(argsJSON) 被触发
        ↓
1. 解析参数 → AgentToolArgs{ Query: "xxx" }
        ↓
2. 构造子 Agent 消息栈
   - 可选注入父历史消息（PassFullContextToSubAgent）
   - 追加 SystemPrompt（子 Agent 身份设定）
   - 追加 User Message（Query）
        ↓
3. 从 ctx 继承关键身份信息
   - parent_session_id（会话继续性）
   - parent_messages_appender（用于将子消息回写父级）
        ↓
4. 构造 StreamEmitter（装饰器模式）
   - 把子 Agent 的 chunk 包装：标记 AgentName、ToolName
   - 透传给父的 emitter
        ↓
5. 调用 t.targetAgent.Run()
   - 触发子 Agent 独立的 ReAct 循环
   - 子 Agent 自主完成：推理 → 调用工具 → 生成回复
   - 父子完全解耦执行
        ↓
6. ReturnFullContextToParent = true 时
   - 从 ctx 读取 parent_messages_appender 回调
   - 将 loopRes.Messages 追加进父消息栈
        ↓
7. 提取 loopRes.Reply，包装成总结字符串返回给父
```

> **关键代码段**：`internal/biz/nocli/openai/agent/agent_tool.go:65`（Run 方法）; `internal/biz/nocli/chat.go:81` 和 `internal/biz/nocli/stream_chat.go:43`（注册回调位置）

---

## 四、三个核心设计点

### 1. 上下文透传（Via `context.Context`）

通过 `ctx.Value` 传递三类信息，不侵入子 Agent 的接口签名：

| Context Key | 类型 | 来源 | 用途 |
|-------------|------|------|------|
| `parent_messages` | `[]openai.ChatCompletionMessage` | 父 Agent | 喂给子 Agent 的历史 |
| `parent_session_id` | `string` | 父 Agent | 保持同会话的 ID 继承 |
| `parent_emitter` | `base.StreamEmitter` | 父 Agent | 子 Agent 流式输出的转发出口 |
| `parent_messages_appender` | `func([]openai.ChatCompletionMessage)` | 父 Agent | 子 Agent 执行完成后回写完整消息历史到父消息栈 |

### 2. StreamEmitter 链式包装（装饰器模式）

```go
subEmitter = func(chunk *pb.StreamChunk) {
    if chunk != nil {
        chunk.AgentName = t.targetAgent.Name()                         // 标注来源 Agent
        if chunk.ToolInfo != nil {
            chunk.ToolInfo.ToolName = fmt.Sprintf("[%s] %s",
                t.targetAgent.Name(), chunk.ToolInfo.ToolName)        // 标注来源 Tool
        }
    }
    parentEmitter(chunk)                                               // 透传给父
}
```

> **作用**：子 Agent 的输出在返回父级之前，被包装上了"身份标签"，实现多 Agent 协作时的**流式输出溯源**。

### 3. Definition 动态生成

```go
Name:        fmt.Sprintf("delegate_to_%s", t.targetAgent.Name())
Description: fmt.Sprintf("将子任务委派给 %s 专门处理。适用场景与能力说明：%s",
    t.targetAgent.Name(), t.targetAgent.Description())
Parameters: {
    "type": "object",
    "properties": {
        "query": { "type": "string", "description": "委派给该 Agent 执行的具体子任务指令" }
    },
    "required": ["query"]
}
```

> 把子 Agent 的描述注入工具定义，让主 Agent 知道**"应该在什么场景下调用我"**。

---

## 五、整体架构图示

```
MainAgent (ReAct Loop)
        │
        │  Function Call: delegate_to_<sub_agent>
        ▼
  AgentTool.Run()
        │
        ├─ 构造子消息栈（注入 SystemPrompt + Query，可选父历史）
        ├─ 包装 StreamEmitter（加上 AgentName / ToolName 标签）
        └─ 调用 IAgent.Run() → 子 Agent 独立 ReAct 循环
                    │
                    ├─ 可能继续调用工具（Tools）
                    ├─ 流式 chunk 经过装饰器返回父级
                    └─ 最终生成 Reply
        │
        ▼ 返回："【子 Agent (xxx) 独立执行总结】\n{reply}"
```

---

## 六、面试高频问题与回答要点

### Q1：AgentTool 解决什么问题？

**答**：单一 Agent 模型能力有限。通过把多种专业 Agent 包装成 Tool，主 Agent 作为 Orchestrator 可以根据用户意图**动态路由、委派任务**，实现"一个大脑调度多个专家"的架构。

---

### Q2：父子 Agent 如何实现上下文隔离又协作？

**答**：通过 `AgentToolOptions` 三个开关精细化控制：
- `PassFullContextToSubAgent=false` → 子 Agent 只看到当前任务，视野清晰，成本低；
- `true` → 子 Agent 能看到之前对话，适合需要全局语境理解的任务；
- 流式装饰器保证父子完全并行执行，不阻塞。

---

### Q3：子 Agent 执行失败怎么处理？

**答**：`Run` 方法对 `targetAgent.Run()` 的错误进行了包装并向上传导：

```go
return "", fmt.Errorf("子 Agent %s 执行失败: %v", t.targetAgent.Name(), err)
```

主 Agent 可以捕获该错误，根据业务策略决定是否重试、回退或向用户报错。

---

### Q4：流式传输时的 AgentName / ToolName 标记有什么用？

**答**：实现多 Agent 混合输出时的**身份溯源**。用户看到流式输出中的 `[nocli] tool: search_devices` 就知道该 chunk 来自哪个子 Agent、哪个工具，避免混淆。

---

### Q5：这和普通 Function Calling 有什么区别？

**答**：本质是 Function Calling 的一种特殊应用，但被调用的"函数体"是一个完整的 Agent 循环（ReAct）：

| 维度 | 普通 Tool | Agent Tool |
|------|-----------|------------|
| 执行内容 | 确定性代码 | 独立推理 → 调用工具 → 生成回复 |
| 决策能力 | 无 | 有（ReAct 多步循环） |
| 返回类型 | 结构化数据 | 自然语言总结 |
| 复杂度 | 低 | 高 |

---

## 七、一句话总结

> **`AgentTool` 是多智能体系统的"遥控器"——它把任何专业 Agent 都包装成一个 OpenAI Function，让主 Agent 能够像调用普通工具一样，委派复杂子任务并实时获取流式中间结果，实现职责分离与能力扩展。**

---

## 八、关联文件（便于深入阅读）

- `internal/biz/nocli/openai/agent/base/` — `IAgent` 接口与 `RunOptions` 定义
- `internal/biz/nocli/openai/agent/react_agent.go` — 子 Agent 的核心 ReAct 循环
- `internal/biz/nocli/openai/chat_model/` — LLM 调用封装
- `ai-rag-demo/api/nocli/v1/` — gRPC 流式 Chunk 结构定义
