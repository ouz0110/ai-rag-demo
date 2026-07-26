<template>
  <div class="chat-view-page">
    <!-- 顶部状态栏 -->
    <header class="chat-header">
      <div class="header-left">
        <span class="status-indicator" :class="statusColorClass"></span>
        <span class="status-title">会话状态: {{ statusText }}</span>
        <span class="session-badge font-mono">({{ sessionId }})</span>
      </div>

      <div class="header-right font-mono">
        <span>AI-RAG Streaming Console</span>
      </div>
    </header>

    <!-- 消息滚动主体区 -->
    <div ref="scrollContainerRef" class="message-scroll-area" @scroll="handleScroll">
      <!-- 历史记录首屏 Skeleton -->
      <div v-if="chatStore.isHistoryLoading" class="loading-box">
        <Loader2 :size="22" class="animate-spin text-indigo-400" />
        <span>正在加载历史记录与上下文切片...</span>
      </div>

      <template v-else>
        <!-- 向上滚动加载更早历史消息的转圈提示 -->
        <div v-if="chatStore.isLoadingMoreHistory" class="top-history-loader">
          <Loader2 :size="15" class="animate-spin text-indigo-400" />
          <span>正在加载更早的历史消息...</span>
        </div>
        <div v-else-if="!chatStore.hasMoreHistory && chatStore.messages.length >= 20" class="top-history-end">
          <span>— 已加载全部历史对话记录 —</span>
        </div>

        <!-- 消息列表 -->
        <ChatMessage v-for="msg in chatStore.messages" :key="msg.id" :msg="msg" />

        <!-- 流式中断与人工授权审批卡片 (无缝嵌入在最后一条消息后) -->
        <InterruptCard
          v-if="chatStore.pendingToolCalls && chatStore.pendingToolCalls.length > 0"
          :pending-calls="chatStore.pendingToolCalls"
          @respond="handleApprovalRespond"
        />

        <!-- 占位平滑滚动锚点 -->
        <div ref="scrollAnchorRef" class="scroll-anchor"></div>
      </template>
    </div>

    <!-- 底部常驻输入框 -->
    <ChatInput />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { Loader2 } from 'lucide-vue-next';
import ChatMessage from '../components/chat/ChatMessage.vue';
import InterruptCard from '../components/chat/InterruptCard.vue';
import ChatInput from '../components/chat/ChatInput.vue';
import { useChatStore } from '../stores/chat';
import { SessionStatus, ResumeAction, ApproveScope } from '../types/api';

const route = useRoute();
const router = useRouter();
const chatStore = useChatStore();

const scrollContainerRef = ref<HTMLDivElement | null>(null);
const scrollAnchorRef = ref<HTMLDivElement | null>(null);

const sessionId = computed(() => {
  const id = (route.params.sessionId as string) || '';
  if (!id || id === 'undefined') return '';
  return id;
});

// 会话状态呈现
const statusText = computed(() => {
  switch (chatStore.sessionStatus) {
    case SessionStatus.SS_RUNNING:
      return '推导处理中...';
    case SessionStatus.SS_INTERRUPTED:
      return '已中断挂起 (等待人工授权审批)';
    case SessionStatus.SS_IDLE:
    default:
      return '就绪';
  }
});

const statusColorClass = computed(() => {
  switch (chatStore.sessionStatus) {
    case SessionStatus.SS_RUNNING:
      return 'bg-running';
    case SessionStatus.SS_INTERRUPTED:
      return 'bg-interrupted';
    case SessionStatus.SS_IDLE:
    default:
      return 'bg-idle';
  }
});

// 监听滚动区顶部触顶事件 (向上懒加载更早的历史消息)
async function handleScroll() {
  const container = scrollContainerRef.value;
  if (!container) return;

  if (
    container.scrollTop <= 30 &&
    chatStore.hasMoreHistory &&
    !chatStore.isLoadingMoreHistory &&
    !chatStore.isHistoryLoading
  ) {
    const previousScrollHeight = container.scrollHeight;
    await chatStore.loadMoreHistory();

    // 维持原有相对滚动位置，避免跳动
    nextTick(() => {
      if (scrollContainerRef.value) {
        const newScrollHeight = scrollContainerRef.value.scrollHeight;
        scrollContainerRef.value.scrollTop = newScrollHeight - previousScrollHeight;
      }
    });
  }
}

// 监听路由变更或切换会话
watch(
  () => route.params.sessionId,
  (newId) => {
    if (!newId || newId === 'undefined') {
      router.replace('/chat');
      return;
    }
    chatStore.selectSession(newId as string);
  },
  { immediate: true }
);

// 监听历史记录首屏加载完成，立刻强制对齐定位到对话最底部
watch(
  () => chatStore.isHistoryLoading,
  (loading) => {
    if (!loading) {
      scrollToBottomImmediate();
    }
  }
);

// 监听中断审批列表 (pendingToolCalls) 产生，立刻向上拉起滚动条完整展示审批卡片
watch(
  () => chatStore.pendingToolCalls,
  (pending) => {
    if (pending && pending.length > 0) {
      scrollToBottomImmediate();
    }
  },
  { deep: true, immediate: true }
);

// 监听新消息产生/增量更新，平滑滚动到底部
watch(
  () => chatStore.messages,
  () => {
    scrollToBottom();
  },
  { deep: true }
);

onMounted(() => {
  if (!sessionId.value || sessionId.value === 'undefined') {
    router.replace('/chat');
    return;
  }
  chatStore.selectSession(sessionId.value);
});

function scrollToBottomImmediate() {
  nextTick(() => {
    const doScroll = () => {
      if (scrollContainerRef.value) {
        scrollContainerRef.value.scrollTop = scrollContainerRef.value.scrollHeight + 1000;
      }
      if (scrollAnchorRef.value) {
        scrollAnchorRef.value.scrollIntoView({ behavior: 'smooth', block: 'end' });
      }
    };
    doScroll();
    setTimeout(doScroll, 80);
    setTimeout(doScroll, 200);
  });
}

function scrollToBottom() {
  nextTick(() => {
    if (scrollContainerRef.value) {
      scrollContainerRef.value.scrollTop = scrollContainerRef.value.scrollHeight + 1000;
    }
  });
}

// 人工审批响应
function handleApprovalRespond(payload: {
  interruptId: string;
  action: ResumeAction;
  approveScope: ApproveScope;
  reason?: string;
}) {
  chatStore.resumeStream(
    payload.interruptId,
    payload.action,
    payload.approveScope,
    payload.reason
  );
}
</script>

<style scoped>
.chat-view-page {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  position: relative;
  overflow: hidden;
  background-color: #080a13;
}

.chat-header {
  height: 48px;
  min-height: 48px;
  padding: 0 1.25rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background-color: rgba(10, 12, 20, 0.9);
  backdrop-filter: blur(12px);
  display: flex;
  align-items: center;
  justify-content: space-between;
  z-index: 15;
  user-select: none;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.bg-running {
  background-color: #34d399;
  box-shadow: 0 0 8px #34d399;
  animation: pulse 1.5s infinite;
}

.bg-interrupted {
  background-color: #fbbf24;
  box-shadow: 0 0 8px #fbbf24;
  animation: pulse 1s infinite;
}

.bg-idle {
  background-color: #64748b;
}

.status-title {
  font-size: 0.8125rem;
  font-weight: 700;
  color: #f1f5f9;
}

.session-badge {
  font-size: 0.7rem;
  color: #64748b;
}

.header-right {
  font-size: 0.75rem;
  color: #64748b;
}

.message-scroll-area {
  flex: 1;
  overflow-y: auto;
  padding: 1.5rem 1.25rem;
  max-width: 900px;
  margin: 0 auto;
  width: 100%;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.loading-box {
  padding: 3rem 1rem;
  text-align: center;
  font-size: 0.8125rem;
  color: #94a3b8;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.top-history-loader {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 0 12px 0;
  font-size: 0.75rem;
  color: #818cf8;
  font-weight: 500;
  user-select: none;
}

.top-history-end {
  text-align: center;
  padding: 8px 0 12px 0;
  font-size: 0.725rem;
  color: #475569;
  letter-spacing: 0.05em;
  user-select: none;
}

.scroll-anchor {
  height: 1rem;
}

.interrupt-sticky-wrapper {
  max-width: 900px;
  width: 100%;
  margin: 0 auto;
  padding: 0 1.25rem 0.5rem 1.25rem;
  box-sizing: border-box;
  z-index: 25;
  flex-shrink: 0;
}

.slide-up-enter-active,
.slide-up-leave-active {
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.slide-up-enter-from,
.slide-up-leave-to {
  opacity: 0;
  transform: translateY(20px);
}
</style>

