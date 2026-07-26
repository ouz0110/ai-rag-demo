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
  role: 'user' | 'assistant' | 'tool';
  content: string;
  reasoning_content: string;
  agent_name: string;
  tools: UIStreamTool[];
  segments: ChatSegment[];
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
    const toolInfo = chunk.tool_info || chunk.toolInfo;

    if (reasoning) {
      msg.reasoning_content += reasoning;
    }

    // 1. 工具响应结果 (role === 'tool')
    if (role === 'tool') {
      if (toolInfo) {
        const tId = toolInfo.tool_call_id || toolInfo.toolCallId || '';
        const tResult = toolInfo.result_preview || toolInfo.resultPreview || '';

        // 更新 msg.tools
        const existingGlobal = msg.tools.find((t) => t.tool_call_id === tId);
        if (existingGlobal) {
          if (tResult) existingGlobal.result_preview = tResult;
          existingGlobal.status = 'completed';
        }

        // 反向查找 segment 中的工具并完成
        for (let i = msg.segments.length - 1; i >= 0; i--) {
          const seg = msg.segments[i];
          if (seg.type === 'tool') {
            const existingInSeg = seg.tools.find((t) => t.tool_call_id === tId);
            if (existingInSeg) {
              if (tResult) existingInSeg.result_preview = tResult;
              existingInSeg.status = 'completed';
              break;
            }
          }
        }
      }
      return;
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
    if (toolInfo) {
      const tId = toolInfo.tool_call_id || toolInfo.toolCallId || '';
      const tName = toolInfo.tool_name || toolInfo.toolName || '';
      const tArgs = toolInfo.arguments || '';
      const tResult = toolInfo.result_preview || toolInfo.resultPreview;

      // 维护 msg.tools
      const existingGlobal = msg.tools.find((x) => x.tool_call_id === tId);
      if (existingGlobal) {
        if (tResult) existingGlobal.result_preview = tResult;
        existingGlobal.status = 'completed';
      } else {
        msg.tools.push({
          tool_call_id: tId,
          tool_name: tName,
          arguments: tArgs,
          result_preview: tResult,
          status: tResult ? 'completed' : 'running',
        });
      }

      // 维护 msg.segments 中的 tool segment
      const lastSeg = msg.segments[msg.segments.length - 1];
      if (lastSeg && lastSeg.type === 'tool') {
        const existingInSeg = lastSeg.tools.find((x) => x.tool_call_id === tId);
        if (existingInSeg) {
          if (tResult) existingInSeg.result_preview = tResult;
          existingInSeg.status = 'completed';
        } else {
          lastSeg.tools.push({
            tool_call_id: tId,
            tool_name: tName,
            arguments: tArgs,
            result_preview: tResult,
            status: tResult ? 'completed' : 'running',
          });
        }
      } else {
        msg.segments.push({
          type: 'tool',
          id: 'tool-' + Date.now() + Math.random(),
          tools: [
            {
              tool_call_id: tId,
              tool_name: tName,
              arguments: tArgs,
              result_preview: tResult,
              status: tResult ? 'completed' : 'running',
            },
          ],
        });
      }
    }
  }

  // 从 chunks 重构前端消息结构 (历史记录与流式回放保持统一的大块+小段结构)
  function reconstructMessagesFromChunks(chunks: any[]): UIChatMessage[] {
    const list: UIChatMessage[] = [];
    let currentAssistantMsg: UIChatMessage | null = null;

    for (const chunk of chunks) {
      const role = chunk.role || 'assistant';

      if (role === 'user') {
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
        if (!currentAssistantMsg) {
          currentAssistantMsg = {
            id: 'assistant-' + Date.now() + Math.random(),
            role: 'assistant',
            content: '',
            reasoning_content: '',
            agent_name: chunk.agent_name || chunk.agentName || 'main',
            tools: [],
            segments: [],
            created_at: Date.now(),
          };
          list.push(currentAssistantMsg);
        }

        const agentName = chunk.agent_name || chunk.agentName;
        if (agentName) {
          currentAssistantMsg.agent_name = agentName;
        }

        appendChunkToMessage(currentAssistantMsg, chunk);
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
      segments: [],
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

    let targetMsg = messages.value[messages.value.length - 1];

    if (!targetMsg || targetMsg.role !== 'assistant') {
      targetMsg = {
        id: 'assistant-' + Date.now() + Math.random(),
        role: 'assistant',
        content: '',
        reasoning_content: '',
        agent_name: chunk.agent_name || chunk.agentName || 'main',
        tools: [],
        segments: [],
        created_at: Date.now(),
        isStreaming: true,
      };
      messages.value.push(targetMsg);
    }

    const agentName = chunk.agent_name || chunk.agentName;
    if (agentName) {
      targetMsg.agent_name = agentName;
    }

    appendChunkToMessage(targetMsg, chunk);

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
