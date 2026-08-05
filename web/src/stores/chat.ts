import { defineStore } from 'pinia';
import { ref } from 'vue';
import { router } from '../router';
import { chatApi } from '../api/chat';
import { fetchSSE } from '../sse/sseClient';
import { useKBStore } from './kb';
import {
  SessionStatus,
  ResumeAction,
  ApproveScope,
  type SessionInfo,
  type StreamChunk,
  type PendingToolCall,
  type CompressInfo,
} from '../types/api';

export interface UIStreamTool {
  tool_call_id: string;
  tool_name: string;
  arguments: string;
  result_preview?: string;
  status: 'running' | 'completed';
  duration_ms?: number;
}

export interface ChatSegmentText {
  type: 'text';
  id: string;
  content: string;
}

export interface ChatSegmentTool {
  type: 'tool';
  id: string;
  tools: UIStreamTool[];
}

export type ChatSegment = ChatSegmentText | ChatSegmentTool;

export interface UIChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'tool' | 'system';
  content: string;
  reasoning_content: string;
  agent_name: string;
  tools: UIStreamTool[];
  segments: ChatSegment[];
  created_at: number;
  isStreaming?: boolean;
  isStopped?: boolean;
  error?: string;
  compress_info?: CompressInfo;
}

export const useChatStore = defineStore('chat', () => {
  // 会话列表
  const sessions = ref<SessionInfo[]>([]);
  const currentSessionId = ref<string>('');
  const sessionStatus = ref<SessionStatus>(SessionStatus.SS_IDLE);

  // 当前会话的消息列表
  const messages = ref<UIChatMessage[]>([]);

  // 待处理的中断工具审批列表
  const pendingToolCalls = ref<PendingToolCall[]>([]);

  // 加载与生成状态
  const isSessionsLoading = ref<boolean>(false);
  const isHistoryLoading = ref<boolean>(false);
  const isGenerating = ref<boolean>(false);

  // 模型选择
  const selectedModel = ref<string>('deepseek-r1');

  // AbortController
  let abortController: AbortController | null = null;

  // 1. 获取会话列表
  async function fetchSessions() {
    isSessionsLoading.value = true;
    try {
      const res: any = await chatApi.listSessions({ page: { number: 1, size: 50 } });
      const rawList = res?.sessions || res?.Sessions || [];
      sessions.value = rawList
        .map((s: any) => ({
          session_id: s.session_id || s.sessionId || s.SessionID || s.SessionId || '',
          name: s.name || s.Name || '新会话',
          status: typeof s.status !== 'undefined' ? s.status : SessionStatus.SS_IDLE,
          created_at: s.created_at || s.createdAt || 0,
          updated_at: s.updated_at || s.updatedAt || 0,
        }))
        .filter((s: SessionInfo) => s.session_id && s.session_id !== 'undefined');
    } catch (err) {
      console.error('Failed to fetch sessions:', err);
    } finally {
      isSessionsLoading.value = false;
    }
  }

  // 历史记录分页懒加载状态
  const hasMoreHistory = ref<boolean>(false);
  const historyPage = ref<number>(1);
  const isLoadingMoreHistory = ref<boolean>(false);

  // 2. 选择/切换会话并加载历史
  async function selectSession(sessionId: string) {
    if (!sessionId || sessionId === 'undefined' || !sessionId.trim()) {
      return;
    }
    if (currentSessionId.value === sessionId && (isGenerating.value || messages.value.length > 0)) {
      return;
    }
    
    currentSessionId.value = sessionId;
    messages.value = [];
    pendingToolCalls.value = [];
    sessionStatus.value = SessionStatus.SS_IDLE;
    isHistoryLoading.value = true;
    historyPage.value = 1;
    hasMoreHistory.value = false;

    try {
      const res: any = await chatApi.getSessionHistory({
        session_id: sessionId,
        page: { number: 1, size: 20 },
      });
      sessionStatus.value = typeof res?.status !== 'undefined' ? res.status : SessionStatus.SS_IDLE;

      hasMoreHistory.value = !!(res?.has_more ?? res?.hasMore);

      const rawPending = res?.pending_tool_calls || res?.pendingToolCalls || res?.PendingToolCalls || [];
      pendingToolCalls.value = rawPending.map((p: any) => ({
        interrupt_id: p.interrupt_id || p.interruptId || p.InterruptId || '',
        tool_call_id: p.tool_call_id || p.toolCallId || p.ToolCallId || '',
        tool_name: p.tool_name || p.toolName || p.ToolName || '',
        arguments: p.arguments || p.Arguments || '',
      }));
      if (pendingToolCalls.value.length > 0) {
        sessionStatus.value = SessionStatus.SS_INTERRUPTED;
      }

      // 回放/解析后端返回的 chunks 重构历史 UI
      const chunks = res?.chunks || res?.Chunks || [];
      if (chunks && chunks.length > 0) {
        messages.value = reconstructMessagesFromChunks(chunks);
      }
    } catch (err) {
      console.error('Failed to load session history:', err);
    } finally {
      isHistoryLoading.value = false;
    }
  }

  // 加载更早的倒序历史消息 (向上滚动懒加载)
  async function loadMoreHistory() {
    if (!currentSessionId.value || !hasMoreHistory.value || isLoadingMoreHistory.value || isGenerating.value) {
      return;
    }

    isLoadingMoreHistory.value = true;
    const nextPage = historyPage.value + 1;

    try {
      const res: any = await chatApi.getSessionHistory({
        session_id: currentSessionId.value,
        page: { number: nextPage, size: 20 },
      });

      historyPage.value = nextPage;
      hasMoreHistory.value = !!(res?.has_more ?? res?.hasMore);

      const chunks = res?.chunks || res?.Chunks || [];
      if (chunks && chunks.length > 0) {
        const olderMessages = reconstructMessagesFromChunks(chunks);
        // 将较旧的历史消息拼接到当前消息的最顶部
        messages.value = [...olderMessages, ...messages.value];
      }
    } catch (err) {
      console.error('Failed to load older session history:', err);
    } finally {
      isLoadingMoreHistory.value = false;
    }
  }

  // 辅助函数: 增量拼接 Chunk 到 assistant 消息的 segments 列表中
  function appendChunkToMessage(msg: UIChatMessage, chunk: any) {
    if (!msg.segments) {
      msg.segments = [];
    }

    const role = chunk.role || 'assistant';
    const text = chunk.text || chunk.Text;
    const reasoning = chunk.reasoning_text || chunk.reasoningText;
    const toolInfo = chunk.tool_info || chunk.toolInfo || chunk.ToolInfo;
    const event = chunk.event;

    if (reasoning) {
      msg.reasoning_content += reasoning;
    }

    // 1. 判断是否为工具响应结果处理 (SET_TOOL_RESULT 或 role === 'tool')
    const isToolResultEvent =
      role === 'tool' ||
      event === 4 ||
      event === 'SET_TOOL_RESULT' ||
      !!(toolInfo && (toolInfo.result_preview !== undefined || toolInfo.resultPreview !== undefined || toolInfo.duration_ms !== undefined || toolInfo.durationMs !== undefined || chunk.duration_ms !== undefined || chunk.durationMs !== undefined));

    if (isToolResultEvent && toolInfo) {
      const tId = toolInfo.tool_call_id || toolInfo.toolCallId || toolInfo.ToolCallId || '';
      const tResult = toolInfo.result_preview || toolInfo.resultPreview || toolInfo.ResultPreview || '';
      const durMs = toolInfo.duration_ms ?? toolInfo.durationMs ?? toolInfo.DurationMs ?? chunk.duration_ms ?? chunk.durationMs ?? chunk.DurationMs;

      if (tId) {
        // 更新 msg.tools 全局数组
        const existingGlobal = msg.tools.find((t) => t.tool_call_id === tId);
        if (existingGlobal) {
          if (tResult) existingGlobal.result_preview = tResult;
          if (durMs !== undefined && durMs !== null && Number(durMs) > 0) existingGlobal.duration_ms = Number(durMs);
          existingGlobal.status = 'completed';
        }

        // 遍历 msg.segments 查找并完成对应工具
        let updatedInSeg = false;
        for (let i = msg.segments.length - 1; i >= 0; i--) {
          const seg = msg.segments[i];
          if (seg.type === 'tool') {
            const existingInSeg = seg.tools.find((t) => t.tool_call_id === tId);
            if (existingInSeg) {
              if (tResult) existingInSeg.result_preview = tResult;
              if (durMs !== undefined && durMs !== null && Number(durMs) > 0) existingInSeg.duration_ms = Number(durMs);
              existingInSeg.status = 'completed';
              updatedInSeg = true;
              break;
            }
          }
        }

        if (!updatedInSeg) {
          const lastSeg = msg.segments[msg.segments.length - 1];
          const newToolObj: UIStreamTool = {
            tool_call_id: tId,
            tool_name: toolInfo.tool_name || toolInfo.toolName || 'tool',
            arguments: toolInfo.arguments || '',
            result_preview: tResult,
            duration_ms: durMs ? Number(durMs) : undefined,
            status: 'completed',
          };
          if (lastSeg && lastSeg.type === 'tool') {
            lastSeg.tools.push(newToolObj);
          } else {
            msg.segments.push({
              type: 'tool',
              id: 'tool-' + Date.now() + Math.random(),
              tools: [newToolObj],
            });
          }
        }
      }
      if (role === 'tool') return;
    }

    // 2. 文本 Delta
    if (text) {
      msg.content += text;
      const lastSeg = msg.segments[msg.segments.length - 1];
      if (lastSeg && lastSeg.type === 'text') {
        lastSeg.content += text;
      } else {
        msg.segments.push({
          type: 'text',
          id: 'text-' + Date.now() + Math.random(),
          content: text,
        });
      }
    }

    // 3. 工具启动 (SET_TOOL_START)
    if (toolInfo && !isToolResultEvent) {
      const tId = toolInfo.tool_call_id || toolInfo.toolCallId || toolInfo.ToolCallId || '';
      const tName = toolInfo.tool_name || toolInfo.toolName || toolInfo.ToolName || '';
      const tArgs = toolInfo.arguments || toolInfo.Arguments || '';
      const tResult = toolInfo.result_preview || toolInfo.resultPreview || toolInfo.ResultPreview;
      const durMs = toolInfo.duration_ms ?? toolInfo.durationMs ?? toolInfo.DurationMs ?? chunk.duration_ms ?? chunk.durationMs ?? chunk.DurationMs;

      if (tId) {
        const existingGlobal = msg.tools.find((x) => x.tool_call_id === tId);
        if (existingGlobal) {
          if (tResult) existingGlobal.result_preview = tResult;
          if (durMs !== undefined && durMs !== null && Number(durMs) > 0) existingGlobal.duration_ms = Number(durMs);
          existingGlobal.status = tResult ? 'completed' : 'running';
        } else {
          msg.tools.push({
            tool_call_id: tId,
            tool_name: tName,
            arguments: tArgs,
            result_preview: tResult,
            duration_ms: durMs ? Number(durMs) : undefined,
            status: tResult ? 'completed' : 'running',
          });
        }

        let foundInSeg = false;
        for (let i = msg.segments.length - 1; i >= 0; i--) {
          const seg = msg.segments[i];
          if (seg.type === 'tool') {
            const existingInSeg = seg.tools.find((x) => x.tool_call_id === tId);
            if (existingInSeg) {
              if (tResult) existingInSeg.result_preview = tResult;
              if (durMs !== undefined && durMs !== null && Number(durMs) > 0) existingInSeg.duration_ms = Number(durMs);
              if (tResult) existingInSeg.status = 'completed';
              foundInSeg = true;
              break;
            }
          }
        }

        if (!foundInSeg) {
          const lastSeg = msg.segments[msg.segments.length - 1];
          const newToolObj: UIStreamTool = {
            tool_call_id: tId,
            tool_name: tName,
            arguments: tArgs,
            result_preview: tResult,
            duration_ms: durMs ? Number(durMs) : undefined,
            status: tResult ? 'completed' : 'running',
          };
          if (lastSeg && lastSeg.type === 'tool') {
            lastSeg.tools.push(newToolObj);
          } else {
            msg.segments.push({
              type: 'tool',
              id: 'tool-' + Date.now() + Math.random(),
              tools: [newToolObj],
            });
          }
        }
      }
    }
  }

  // 从 chunks 重构前端消息结构 (历史记录与流式回放保持统一的大块+小段结构)
  function reconstructMessagesFromChunks(chunks: any[]): UIChatMessage[] {
    const list: UIChatMessage[] = [];
    let currentAssistantMsg: UIChatMessage | null = null;

    for (const chunk of chunks) {
      const role = chunk.role || 'assistant';
      const event = chunk.event;

      if (event === 8 || event === 'SET_CONTEXT_COMPRESSED' || role === 'system') {
        currentAssistantMsg = null;
        list.push({
          id: 'compress-' + Date.now() + Math.random(),
          role: 'system',
          content: chunk.text || chunk.Text || '',
          reasoning_content: '',
          agent_name: '',
          tools: [],
          segments: [],
          created_at: Date.now(),
          compress_info: chunk.compress_info || chunk.compressInfo,
        });
      } else if (role === 'user') {
        currentAssistantMsg = null;
        list.push({
          id: 'user-' + Date.now() + Math.random(),
          role: 'user',
          content: chunk.text || chunk.Text || '',
          reasoning_content: '',
          agent_name: '',
          tools: [],
          segments: [],
          created_at: Date.now(),
        });
      } else {
        const agentName = chunk.agent_name || chunk.agentName || 'main';
        if (!currentAssistantMsg || currentAssistantMsg.agent_name !== agentName) {
          currentAssistantMsg = {
            id: 'assistant-' + Date.now() + Math.random(),
            role: 'assistant',
            content: '',
            reasoning_content: '',
            agent_name: agentName,
            tools: [],
            segments: [],
            created_at: Date.now(),
          };
          list.push(currentAssistantMsg);
        }
        appendChunkToMessage(currentAssistantMsg, chunk);
      }
    }

    if (list.length > 0) {
      const last = list[list.length - 1];
      if (last.role === 'assistant') {
        last.isStopped = true;
      }
    }

    return list;
  }

  // 3. 发送新消息 (Stream Completion)
  async function sendMessage(text: string) {
    if (!text.trim() || isGenerating.value) return;

    // 若无会话 ID，先设定一个临时 session_id 方便立即切页展示
    if (!currentSessionId.value) {
      currentSessionId.value = 'temp-' + Date.now();
    }

    // 立马插入用户消息
    const userMsg: UIChatMessage = {
      id: 'user-' + Date.now(),
      role: 'user',
      content: text,
      reasoning_content: '',
      agent_name: '',
      tools: [],
      segments: [],
      created_at: Date.now(),
    };
    messages.value.push(userMsg);

    // 立马插入 Agent 响应占位消息（携带 isStreaming: true 以展现 loading 效果）
    const assistantMsg: UIChatMessage = {
      id: 'assistant-' + Date.now(),
      role: 'assistant',
      content: '',
      reasoning_content: '',
      agent_name: 'main',
      tools: [],
      segments: [],
      created_at: Date.now(),
      isStreaming: true,
    };
    messages.value.push(assistantMsg);

    isGenerating.value = true;
    pendingToolCalls.value = [];
    sessionStatus.value = SessionStatus.SS_RUNNING;

    // 若当前在 /chat 首页，立即路由跳转到对话详情视图
    if (router.currentRoute.value.path === '/chat' || router.currentRoute.value.path === '/chat/') {
      router.push(`/chat/${currentSessionId.value}`);
    }

    abortController = new AbortController();

    const kbStore = useKBStore();

    await fetchSSE({
      url: '/nocli/v1/stream/completion',
      body: {
        message: text,
        session_id: currentSessionId.value.startsWith('temp-') ? '' : currentSessionId.value,
        model: selectedModel.value,
        kb_tenant_id: kbStore.activeKbTenantId,
        kb_id: kbStore.activeKbId,
        enable_rag: kbStore.enableRAG,
        enable_skill: kbStore.enableSkill,
        enable_mcp: kbStore.enableMCP,
        enable_rerank: kbStore.enableRerank,
        agent_tool_options: kbStore.agentToolOptions,
      },
      signal: abortController.signal,
      onChunk: (chunk: StreamChunk) => {
        handleStreamChunk(chunk);
      },
      onError: (err: Error) => {
        const lastMsg = messages.value[messages.value.length - 1];
        if (lastMsg) {
          lastMsg.error = err.message || '输出发生异常';
          lastMsg.isStreaming = false;
        }
        isGenerating.value = false;
        sessionStatus.value = SessionStatus.SS_IDLE;
      },
      onDone: () => {
        const lastMsg = messages.value[messages.value.length - 1];
        if (lastMsg) lastMsg.isStreaming = false;
        isGenerating.value = false;
        fetchSessions(); // 刷新会话列表
      },
    });
  }

  // 4. 恢复中断继续对话 (Stream Resume)
  async function resumeStream(
    interruptId: string,
    action: ResumeAction,
    approveScope: ApproveScope,
    reason?: string
  ) {
    if (isGenerating.value) return;

    // 清理尾部残留的空占位符
    const last = messages.value[messages.value.length - 1];
    if (
      last &&
      last.role === 'assistant' &&
      !last.content &&
      !last.reasoning_content &&
      (!last.tools || last.tools.length === 0) &&
      (!last.segments || last.segments.length === 0)
    ) {
      messages.value.pop();
    }

    isGenerating.value = true;
    pendingToolCalls.value = [];
    sessionStatus.value = SessionStatus.SS_RUNNING;

    abortController = new AbortController();

    const kbStore = useKBStore();

    await fetchSSE({
      url: '/nocli/v1/stream/resume',
      body: {
        session_id: currentSessionId.value,
        interrupt_id: interruptId,
        action,
        approve_scope: approveScope,
        reason,
        model: selectedModel.value,
        kb_tenant_id: kbStore.activeKbTenantId,
        kb_id: kbStore.activeKbId,
        enable_rag: kbStore.enableRAG,
        enable_skill: kbStore.enableSkill,
        enable_mcp: kbStore.enableMCP,
        enable_rerank: kbStore.enableRerank,
        agent_tool_options: kbStore.agentToolOptions,
      },
      signal: abortController.signal,
      onChunk: (chunk: StreamChunk) => {
        handleStreamChunk(chunk);
      },
      onError: (err: Error) => {
        const lastMsg = messages.value[messages.value.length - 1];
        if (lastMsg) {
          lastMsg.error = err.message || '恢复响应发生错误';
          lastMsg.isStreaming = false;
        }
        isGenerating.value = false;
        sessionStatus.value = SessionStatus.SS_IDLE;
      },
      onDone: () => {
        const lastMsg = messages.value[messages.value.length - 1];
        if (lastMsg) lastMsg.isStreaming = false;
        isGenerating.value = false;
        fetchSessions();
      },
    });
  }

  // 统一处理 SSE Chunk 核心逻辑
  function handleStreamChunk(chunk: any) {
    const sId = chunk.session_id || chunk.sessionId;
    if (sId && sId !== 'undefined') {
      const isNewSession =
        !currentSessionId.value ||
        currentSessionId.value.startsWith('temp-') ||
        currentSessionId.value !== sId;
      const oldSessionId = currentSessionId.value;
      currentSessionId.value = sId;

      if (isNewSession) {
        fetchSessions();
        if (
          router.currentRoute.value.path === '/chat' ||
          router.currentRoute.value.path === '/chat/' ||
          router.currentRoute.value.path === `/chat/${oldSessionId}`
        ) {
          router.replace(`/chat/${sId}`);
        }
      }
    }

    if (typeof chunk.status !== 'undefined') {
      sessionStatus.value = chunk.status;
    }

    // 中断事件 / 待确认工具调用处理 (优先提取 pending_tool_calls)
    const pending = chunk.pending_tool_calls || chunk.pendingToolCalls;
    if (pending && pending.length > 0) {
      pendingToolCalls.value = pending.map((p: any) => ({
        interrupt_id: p.interrupt_id || p.interruptId || '',
        tool_call_id: p.tool_call_id || p.toolCallId || '',
        tool_name: p.tool_name || p.toolName || '',
        arguments: p.arguments || '',
        agent_name: p.agent_name || p.agentName || '',
      }));
      sessionStatus.value = SessionStatus.SS_INTERRUPTED;
    }

    const event = chunk.event;
    if (event === 8 || event === 'SET_CONTEXT_COMPRESSED') {
      const info = chunk.compress_info || chunk.compressInfo;
      const count = info?.compress_count ?? (info as any)?.compressCount ?? 0;

      // 1. 根据 compress_count 精确寻找匹配的 system 压缩卡片 (避免全局覆盖)
      let existingCompressIndex = -1;
      for (let i = messages.value.length - 1; i >= 0; i--) {
        const m = messages.value[i];
        if (m.role === 'system' && m.compress_info) {
          const mCount = m.compress_info.compress_count ?? (m.compress_info as any)?.compressCount ?? 0;
          if (mCount === count && count > 0) {
            existingCompressIndex = i;
            break;
          }
        }
      }

      if (existingCompressIndex !== -1) {
        // 更新对应批次的压缩卡片内容与压缩信息
        const existingMsg = messages.value[existingCompressIndex];
        if (chunk.text || chunk.Text) {
          existingMsg.content = chunk.text || chunk.Text;
        }
        if (info) {
          existingMsg.compress_info = info;
        }
      } else {
        // 创建新的批次 system 压缩卡片 (多组压缩在对应位置独立呈现)
        const compressMsg: UIChatMessage = {
          id: 'compress-' + (count || Date.now()) + '-' + Math.random(),
          role: 'system',
          content: chunk.text || chunk.Text || '',
          reasoning_content: '',
          agent_name: '',
          tools: [],
          segments: [],
          created_at: Date.now(),
          compress_info: info,
        };

        // 🎯 关键逻辑：若末尾是空 Assistant 占位消息 (isStreaming: true 且无内容)，则将压缩卡片插在占位消息前
        const lastMsg = messages.value[messages.value.length - 1];
        if (
          lastMsg &&
          lastMsg.role === 'assistant' &&
          !lastMsg.content &&
          !lastMsg.reasoning_content &&
          (!lastMsg.segments || lastMsg.segments.length === 0)
        ) {
          messages.value.splice(messages.value.length - 1, 0, compressMsg);
        } else {
          messages.value.push(compressMsg);
        }
      }
      return;
    }

    let targetMsg = messages.value[messages.value.length - 1];
    const agentName = chunk.agent_name || chunk.agentName || 'main';

    // 🎯 检查当前 Chunk 是否包含实际输出 Payload
    const hasPayload =
      Boolean(chunk.text || chunk.Text) ||
      Boolean(chunk.reasoning_text || chunk.reasoningText) ||
      Boolean(chunk.tool_info || chunk.toolInfo) ||
      chunk.role === 'tool';

    if (
      targetMsg &&
      targetMsg.role === 'assistant' &&
      !targetMsg.content &&
      !targetMsg.reasoning_content &&
      (!targetMsg.tools || targetMsg.tools.length === 0) &&
      (!targetMsg.segments || targetMsg.segments.length === 0)
    ) {
      // 重用末尾空占位消息，更新为当前 Chunk 的 agent_name
      targetMsg.agent_name = agentName;
    } else if (
      !targetMsg ||
      targetMsg.role !== 'assistant' ||
      (targetMsg.agent_name && targetMsg.agent_name !== agentName)
    ) {
      // 若无 Payload 且属于纯控制事件 (如 SET_INTERRUPT / SET_DONE / SET_ERROR)，无需拉起空消息卡片
      if (!hasPayload && (event === 5 || event === 'SET_INTERRUPT' || event === 6 || event === 'SET_DONE')) {
        return;
      }

      targetMsg = {
        id: 'assistant-' + Date.now() + Math.random(),
        role: 'assistant',
        content: '',
        reasoning_content: '',
        agent_name: agentName,
        tools: [],
        segments: [],
        created_at: Date.now(),
        isStreaming: true,
      };
      messages.value.push(targetMsg);
    } else if (agentName && !targetMsg.agent_name) {
      targetMsg.agent_name = agentName;
    }

    appendChunkToMessage(targetMsg, chunk);

    // 错误判定
    if (chunk.error) {
      targetMsg.error = chunk.error.message;
    }
  }

  // 手动中断流 (用户点击停止生成)
  async function stopGeneration() {
    const sessionIdToStop = currentSessionId.value;
    if (abortController) {
      abortController.abort();
      abortController = null;
    }
    isGenerating.value = false;
    sessionStatus.value = SessionStatus.SS_PAUSED;
    const lastAssistantMsg = [...messages.value].reverse().find(m => m.role === 'assistant');
    if (lastAssistantMsg) {
      lastAssistantMsg.isStreaming = false;
      lastAssistantMsg.isStopped = true;
    }
    // 🎯 触发 Vue 3 Ref 响应式更新，确保 computed 立即重算并渲染【继续生成】按钮
    messages.value = [...messages.value];

    // 🎯 显式向后端发送 StopSession 接口，通知后端立刻感知并 cancel() 正在运行的任务
    if (sessionIdToStop) {
      try {
        await chatApi.stopSession({ session_id: sessionIdToStop });
      } catch (err) {
        console.warn('通知后端停止任务失败:', err);
      }
    }
  }

  // 🎯 继续生成 (Continue Generation)
  async function continueGeneration() {
    if (isGenerating.value || !currentSessionId.value) return;

    const lastMsg = messages.value[messages.value.length - 1];
    if (lastMsg && lastMsg.role === 'assistant') {
      lastMsg.isStopped = false;
      lastMsg.isStreaming = true;
    }

    isGenerating.value = true;
    sessionStatus.value = SessionStatus.SS_RUNNING;
    abortController = new AbortController();

    const kbStore = useKBStore();

    await fetchSSE({
      url: '/nocli/v1/stream/completion',
      body: {
        message: '',
        session_id: currentSessionId.value,
        is_continue: true,
        model: selectedModel.value,
        kb_tenant_id: kbStore.activeKbTenantId,
        kb_id: kbStore.activeKbId,
        enable_rag: kbStore.enableRAG,
        enable_skill: kbStore.enableSkill,
        enable_mcp: kbStore.enableMCP,
        enable_rerank: kbStore.enableRerank,
        agent_tool_options: kbStore.agentToolOptions,
      },
      signal: abortController.signal,
      onChunk: (chunk: StreamChunk) => {
        handleStreamChunk(chunk);
      },
      onError: (err: Error) => {
        const last = messages.value[messages.value.length - 1];
        if (last) {
          last.error = err.message || '继续生成发生错误';
          last.isStreaming = false;
        }
        isGenerating.value = false;
        sessionStatus.value = SessionStatus.SS_IDLE;
      },
      onDone: () => {
        const last = messages.value[messages.value.length - 1];
        if (last) {
          last.isStreaming = false;
          last.isStopped = false;
        }
        isGenerating.value = false;
        fetchSessions();
      },
    });
  }

  // 删除会话
  async function deleteSession(sessionId: string) {
    await chatApi.deleteSession({ session_id: sessionId });
    if (currentSessionId.value === sessionId) {
      currentSessionId.value = '';
      messages.value = [];
      pendingToolCalls.value = [];
    }
    await fetchSessions();
  }

  // 清空当前面板新建会话
  function resetCurrentChat() {
    currentSessionId.value = '';
    messages.value = [];
    pendingToolCalls.value = [];
    sessionStatus.value = SessionStatus.SS_IDLE;
  }

  return {
    sessions,
    currentSessionId,
    sessionStatus,
    messages,
    pendingToolCalls,
    isSessionsLoading,
    isHistoryLoading,
    hasMoreHistory,
    historyPage,
    isLoadingMoreHistory,
    isGenerating,
    selectedModel,
    fetchSessions,
    selectSession,
    loadMoreHistory,
    sendMessage,
    resumeStream,
    stopGeneration,
    continueGeneration,
    deleteSession,
    resetCurrentChat,
  };
});
