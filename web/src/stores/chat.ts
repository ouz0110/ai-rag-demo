import { defineStore } from 'pinia';
import { ref } from 'vue';
import { router } from '../router';
import { chatApi } from '../api/chat';
import { fetchSSE } from '../sse/sseClient';
import {
  SessionStatus,
  ResumeAction,
  ApproveScope,
  type SessionInfo,
  type StreamChunk,
  type PendingToolCall,
} from '../types/api';

export interface UIStreamTool {
  tool_call_id: string;
  tool_name: string;
  arguments: string;
  result_preview?: string;
  status: 'running' | 'completed';
}

export interface UIChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'tool';
  content: string;
  reasoning_content: string;
  agent_name: string;
  tools: UIStreamTool[];
  created_at: number;
  isStreaming?: boolean;
  error?: string;
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

      const rawPending = res?.pending_tool_calls || res?.pendingToolCalls || [];
      pendingToolCalls.value = rawPending.map((p: any) => ({
        interrupt_id: p.interrupt_id || p.interruptId || '',
        tool_call_id: p.tool_call_id || p.toolCallId || '',
        tool_name: p.tool_name || p.toolName || '',
        arguments: p.arguments || '',
      }));

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

  // 从 chunks 重构前端消息结构 (实现 user -> assistant -> tool -> assistant 时间线交织)
  function reconstructMessagesFromChunks(chunks: any[]): UIChatMessage[] {
    const list: UIChatMessage[] = [];
    let currentAssistantMsg: UIChatMessage | null = null;
    let currentAssistantToolsCompleted = false;

    for (const chunk of chunks) {
      const role = chunk.role || 'assistant';

      if (role === 'user') {
        currentAssistantMsg = null;
        currentAssistantToolsCompleted = false;
        list.push({
          id: 'user-' + Date.now() + Math.random(),
          role: 'user',
          content: chunk.text || chunk.Text || '',
          reasoning_content: '',
          agent_name: '',
          tools: [],
          created_at: Date.now(),
        });
      } else if (role === 'tool') {
        const toolInfo = chunk.tool_info || chunk.toolInfo;
        if (toolInfo && currentAssistantMsg) {
          const tId = toolInfo.tool_call_id || toolInfo.toolCallId || '';
          const tResult = toolInfo.result_preview || toolInfo.resultPreview || '';
          const existingTool = currentAssistantMsg.tools.find((x) => x.tool_call_id === tId);
          if (existingTool) {
            existingTool.result_preview = tResult;
            existingTool.status = 'completed';
          }
        }
        currentAssistantToolsCompleted = true;
      } else {
        const text = chunk.text || chunk.Text;
        const reasoning = chunk.reasoning_text || chunk.reasoningText;
        const toolInfo = chunk.tool_info || chunk.toolInfo;

        // 如果前一个 assistant 步骤的工具执行完毕，且收到了新一轮文本/思考，开启新的 assistant 消息卡片
        if (!currentAssistantMsg || (currentAssistantToolsCompleted && (text || reasoning))) {
          currentAssistantMsg = {
            id: 'assistant-' + Date.now() + Math.random(),
            role: 'assistant',
            content: '',
            reasoning_content: '',
            agent_name: chunk.agent_name || chunk.agentName || 'main',
            tools: [],
            created_at: Date.now(),
          };
          list.push(currentAssistantMsg);
          currentAssistantToolsCompleted = false;
        }

        const agentName = chunk.agent_name || chunk.agentName;
        if (agentName) {
          currentAssistantMsg.agent_name = agentName;
        }

        if (reasoning) {
          currentAssistantMsg.reasoning_content += reasoning;
        }

        if (text) {
          currentAssistantMsg.content += text;
        }

        if (toolInfo) {
          const tId = toolInfo.tool_call_id || toolInfo.toolCallId || '';
          const tName = toolInfo.tool_name || toolInfo.toolName || '';
          const tArgs = toolInfo.arguments || '';
          const tResult = toolInfo.result_preview || toolInfo.resultPreview;

          const existingTool = currentAssistantMsg.tools.find(
            (item) => item.tool_call_id === tId
          );
          if (existingTool) {
            if (tResult) existingTool.result_preview = tResult;
            existingTool.status = 'completed';
          } else {
            currentAssistantMsg.tools.push({
              tool_call_id: tId,
              tool_name: tName,
              arguments: tArgs,
              result_preview: tResult,
              status: tResult ? 'completed' : 'running',
            });
          }
        }
      }
    }

    return list;
  }

  // 3. 发送新消息 (Stream Completion)
  async function sendMessage(text: string) {
    if (!text.trim() || isGenerating.value) return;

    // 先插入用户消息
    const userMsg: UIChatMessage = {
      id: 'user-' + Date.now(),
      role: 'user',
      content: text,
      reasoning_content: '',
      agent_name: '',
      tools: [],
      created_at: Date.now(),
    };
    messages.value.push(userMsg);

    isGenerating.value = true;
    pendingToolCalls.value = [];
    sessionStatus.value = SessionStatus.SS_RUNNING;

    abortController = new AbortController();

    await fetchSSE({
      url: '/nocli/v1/stream/completion',
      body: {
        message: text,
        session_id: currentSessionId.value,
        model: selectedModel.value,
      },
      signal: abortController.signal,
      onChunk: (chunk: StreamChunk) => {
        handleStreamChunk(chunk);
      },
      onError: (err: Error) => {
        const lastMsg = messages.value[messages.value.length - 1];
        if (lastMsg) lastMsg.error = err.message || '输出发生异常';
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

    isGenerating.value = true;
    pendingToolCalls.value = [];
    sessionStatus.value = SessionStatus.SS_RUNNING;

    abortController = new AbortController();

    await fetchSSE({
      url: '/nocli/v1/stream/resume',
      body: {
        session_id: currentSessionId.value,
        interrupt_id: interruptId,
        action,
        approve_scope: approveScope,
        reason,
        model: selectedModel.value,
      },
      signal: abortController.signal,
      onChunk: (chunk: StreamChunk) => {
        handleStreamChunk(chunk);
      },
      onError: (err: Error) => {
        const lastMsg = messages.value[messages.value.length - 1];
        if (lastMsg) lastMsg.error = err.message || '恢复响应发生错误';
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

  let activeToolsCompleted = false;

  // 统一处理 SSE Chunk 核心逻辑
  function handleStreamChunk(chunk: any) {
    const sId = chunk.session_id || chunk.sessionId;
    if (sId && sId !== 'undefined') {
      const isNewSession = !currentSessionId.value || currentSessionId.value !== sId;
      currentSessionId.value = sId;

      if (isNewSession) {
        fetchSessions();
        if (router.currentRoute.value.path === '/chat' || router.currentRoute.value.path === '/chat/') {
          router.push(`/chat/${sId}`);
        }
      }
    }

    if (typeof chunk.status !== 'undefined') {
      sessionStatus.value = chunk.status;
    }

    const chunkRole = chunk.role || 'assistant';

    // 1. 工具响应结果
    if (chunkRole === 'tool') {
      const toolInfo = chunk.tool_info || chunk.toolInfo;
      if (toolInfo) {
        const tId = toolInfo.tool_call_id || toolInfo.toolCallId || '';
        const tResult = toolInfo.result_preview || toolInfo.resultPreview;

        for (let i = messages.value.length - 1; i >= 0; i--) {
          const m = messages.value[i];
          if (m.role === 'assistant') {
            const existing = m.tools.find((t) => t.tool_call_id === tId);
            if (existing) {
              if (tResult) existing.result_preview = tResult;
              existing.status = 'completed';
              break;
            }
          }
        }
      }
      activeToolsCompleted = true;
      return;
    }

    // 2. assistant 消息处理
    const text = chunk.text || chunk.Text;
    const reasoning = chunk.reasoning_text || chunk.reasoningText;
    const toolInfo = chunk.tool_info || chunk.toolInfo;

    let targetMsg = messages.value[messages.value.length - 1];

    if (!targetMsg || targetMsg.role !== 'assistant' || (activeToolsCompleted && (text || reasoning))) {
      if (targetMsg && targetMsg.role === 'assistant') {
        targetMsg.isStreaming = false;
      }
      targetMsg = {
        id: 'assistant-' + Date.now() + Math.random(),
        role: 'assistant',
        content: '',
        reasoning_content: '',
        agent_name: chunk.agent_name || chunk.agentName || 'main',
        tools: [],
        created_at: Date.now(),
        isStreaming: true,
      };
      messages.value.push(targetMsg);
      activeToolsCompleted = false;
    }

    const agentName = chunk.agent_name || chunk.agentName;
    if (agentName) {
      targetMsg.agent_name = agentName;
    }

    if (reasoning) {
      targetMsg.reasoning_content += reasoning;
    }

    if (text) {
      targetMsg.content += text;
    }

    if (toolInfo) {
      const tId = toolInfo.tool_call_id || toolInfo.toolCallId || '';
      const tName = toolInfo.tool_name || toolInfo.toolName || '';
      const tArgs = toolInfo.arguments || '';
      const tResult = toolInfo.result_preview || toolInfo.resultPreview;

      const existing = targetMsg.tools.find((x) => x.tool_call_id === tId);
      if (existing) {
        if (tResult) existing.result_preview = tResult;
        existing.status = 'completed';
      } else {
        targetMsg.tools.push({
          tool_call_id: tId,
          tool_name: tName,
          arguments: tArgs,
          result_preview: tResult,
          status: tResult ? 'completed' : 'running',
        });
      }
    }

    // 中断事件 / 待确认工具调用
    const pending = chunk.pending_tool_calls || chunk.pendingToolCalls;
    if (pending && pending.length > 0) {
      pendingToolCalls.value = pending.map((p: any) => ({
        interrupt_id: p.interrupt_id || p.interruptId || '',
        tool_call_id: p.tool_call_id || p.toolCallId || '',
        tool_name: p.tool_name || p.toolName || '',
        arguments: p.arguments || '',
      }));
      sessionStatus.value = SessionStatus.SS_INTERRUPTED;
    }

    // 错误判定
    if (chunk.error) {
      targetMsg.error = chunk.error.message;
    }
  }

  // 手动中断流
  function stopGeneration() {
    if (abortController) {
      abortController.abort();
      abortController = null;
    }
    isGenerating.value = false;
    const lastMsg = messages.value[messages.value.length - 1];
    if (lastMsg) lastMsg.isStreaming = false;
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
    deleteSession,
    resetCurrentChat,
  };
});
