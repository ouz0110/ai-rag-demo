import { defineStore } from 'pinia';
import { ref } from 'vue';
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
      const res = await chatApi.listSessions({ page: 1, page_size: 50 });
      sessions.value = res.sessions || [];
    } catch (err) {
      console.error('Failed to fetch sessions:', err);
    } finally {
      isSessionsLoading.value = false;
    }
  }

  // 2. 选择/切换会话并加载历史
  async function selectSession(sessionId: string) {
    if (currentSessionId.value === sessionId && messages.value.length > 0) return;
    
    currentSessionId.value = sessionId;
    messages.value = [];
    pendingToolCalls.value = [];
    sessionStatus.value = SessionStatus.SS_IDLE;
    isHistoryLoading.value = true;

    try {
      const res = await chatApi.getSessionHistory({ session_id: sessionId });
      sessionStatus.value = res.status || SessionStatus.SS_IDLE;
      pendingToolCalls.value = res.pending_tool_calls || [];

      // 回放/解析后端返回的 chunks 重构历史 UI
      if (res.chunks && res.chunks.length > 0) {
        messages.value = reconstructMessagesFromChunks(res.chunks);
      }
    } catch (err) {
      console.error('Failed to load session history:', err);
    } finally {
      isHistoryLoading.value = false;
    }
  }

  // 从 chunks 重构前端消息结构
  function reconstructMessagesFromChunks(chunks: StreamChunk[]): UIChatMessage[] {
    const list: UIChatMessage[] = [];
    let currentAssistantMsg: UIChatMessage | null = null;

    for (const chunk of chunks) {
      const role = chunk.role || 'assistant';

      if (role === 'user') {
        currentAssistantMsg = null;
        list.push({
          id: 'user-' + Date.now() + Math.random(),
          role: 'user',
          content: chunk.text || '',
          reasoning_content: '',
          agent_name: '',
          tools: [],
          created_at: Date.now(),
        });
      } else {
        // assistant 消息拼接
        if (!currentAssistantMsg) {
          currentAssistantMsg = {
            id: 'assistant-' + Date.now() + Math.random(),
            role: 'assistant',
            content: '',
            reasoning_content: '',
            agent_name: chunk.agent_name || 'main',
            tools: [],
            created_at: Date.now(),
          };
          list.push(currentAssistantMsg);
        }

        if (chunk.agent_name) {
          currentAssistantMsg.agent_name = chunk.agent_name;
        }

        if (chunk.reasoning_text) {
          currentAssistantMsg.reasoning_content += chunk.reasoning_text;
        }

        if (chunk.text) {
          currentAssistantMsg.content += chunk.text;
        }

        if (chunk.tool_info) {
          const t = chunk.tool_info;
          const existingTool = currentAssistantMsg.tools.find(
            (item) => item.tool_call_id === t.tool_call_id
          );
          if (existingTool) {
            if (t.result_preview) existingTool.result_preview = t.result_preview;
            existingTool.status = 'completed';
          } else {
            currentAssistantMsg.tools.push({
              tool_call_id: t.tool_call_id,
              tool_name: t.tool_name,
              arguments: t.arguments,
              result_preview: t.result_preview,
              status: t.result_preview ? 'completed' : 'running',
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

    // 预创助手消息
    const assistantMsg: UIChatMessage = {
      id: 'assistant-' + Date.now(),
      role: 'assistant',
      content: '',
      reasoning_content: '',
      agent_name: 'main',
      tools: [],
      created_at: Date.now(),
      isStreaming: true,
    };
    messages.value.push(assistantMsg);

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
        handleStreamChunk(chunk, assistantMsg);
      },
      onError: (err: Error) => {
        assistantMsg.error = err.message || '输出发生异常';
        assistantMsg.isStreaming = false;
        isGenerating.value = false;
        sessionStatus.value = SessionStatus.SS_IDLE;
      },
      onDone: () => {
        assistantMsg.isStreaming = false;
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

    // 找到或创建一个助手消息装载后续增量
    let assistantMsg = messages.value[messages.value.length - 1];
    if (!assistantMsg || assistantMsg.role !== 'assistant') {
      assistantMsg = {
        id: 'assistant-' + Date.now(),
        role: 'assistant',
        content: '',
        reasoning_content: '',
        agent_name: 'main',
        tools: [],
        created_at: Date.now(),
        isStreaming: true,
      };
      messages.value.push(assistantMsg);
    } else {
      assistantMsg.isStreaming = true;
    }

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
        handleStreamChunk(chunk, assistantMsg!);
      },
      onError: (err: Error) => {
        assistantMsg!.error = err.message || '恢复响应发生错误';
        assistantMsg!.isStreaming = false;
        isGenerating.value = false;
        sessionStatus.value = SessionStatus.SS_IDLE;
      },
      onDone: () => {
        assistantMsg!.isStreaming = false;
        isGenerating.value = false;
        fetchSessions();
      },
    });
  }

  // 统一处理 SSE Chunk 核心逻辑
  function handleStreamChunk(chunk: StreamChunk, msg: UIChatMessage) {
    if (chunk.session_id) {
      currentSessionId.value = chunk.session_id;
    }

    if (chunk.agent_name) {
      msg.agent_name = chunk.agent_name;
    }

    if (chunk.status) {
      sessionStatus.value = chunk.status;
    }

    // 思考过程增量
    if (chunk.reasoning_text) {
      msg.reasoning_content += chunk.reasoning_text;
    }

    // 文本回答增量
    if (chunk.text) {
      msg.content += chunk.text;
    }

    // 自动工具日志
    if (chunk.tool_info) {
      const t = chunk.tool_info;
      const existing = msg.tools.find((x) => x.tool_call_id === t.tool_call_id);
      if (existing) {
        if (t.result_preview) existing.result_preview = t.result_preview;
        existing.status = 'completed';
      } else {
        msg.tools.push({
          tool_call_id: t.tool_call_id,
          tool_name: t.tool_name,
          arguments: t.arguments,
          result_preview: t.result_preview,
          status: t.result_preview ? 'completed' : 'running',
        });
      }
    }

    // 中断事件 / 待确认工具调用
    if (chunk.pending_tool_calls && chunk.pending_tool_calls.length > 0) {
      pendingToolCalls.value = chunk.pending_tool_calls;
      sessionStatus.value = SessionStatus.SS_INTERRUPTED;
    }

    // 错误判定
    if (chunk.error) {
      msg.error = chunk.error.message;
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
    isGenerating,
    selectedModel,
    fetchSessions,
    selectSession,
    sendMessage,
    resumeStream,
    stopGeneration,
    deleteSession,
    resetCurrentChat,
  };
});
