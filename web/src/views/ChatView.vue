<template>
  <div class="chat-view-page h-full flex flex-col justify-between overflow-hidden relative">
    <!-- 顶部状态栏 -->
    <header class="h-12 border-b border-slate-800/80 px-4 flex items-center justify-between bg-[#131419]/90 backdrop-blur-md shrink-0 z-10 select-none">
      <div class="flex items-center gap-2">
        <span class="w-2 h-2 rounded-full" :class="statusColorClass"></span>
        <span class="text-xs font-semibold text-slate-200">
          会话状态: {{ statusText }}
        </span>
        <span class="text-[11px] text-slate-500 font-mono">({{ sessionId }})</span>
      </div>

      <div class="text-xs text-slate-400 font-mono">
        DeepSeek Streaming Model
      </div>
    </header>

    <!-- 消息滚动主体区 -->
    <div class="flex-1 overflow-y-auto p-4 md:p-6 space-y-4 max-w-4xl mx-auto w-full">
      <!-- 历史记录加载 Skeleton -->
      <div v-if="chatStore.isHistoryLoading" class="p-8 text-center text-xs text-slate-400 flex flex-col items-center gap-2">
        <Loader2 :size="20" class="animate-spin text-indigo-400" />
        <span>正在加载历史记录与上下文切片...</span>
      </div>

      <template v-else>
        <!-- 消息列表 -->
        <ChatMessage v-for="msg in chatStore.messages" :key="msg.id" :msg="msg" />

        <!-- 流式中断与人工授权审批卡片 (处于 SS_INTERRUPTED 或 Pending 状态) -->
        <InterruptCard
          v-if="chatStore.sessionStatus === SessionStatus.SS_INTERRUPTED && chatStore.pendingToolCalls.length > 0"
          :pending-calls="chatStore.pendingToolCalls"
          @respond="handleApprovalRespond"
        />

        <!-- 占位平滑滚动锚点 -->
        <div ref="scrollAnchorRef" class="h-4"></div>
      </template>
    </div>

    <!-- 底部常驻输入框 -->
    <ChatInput />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue';
import { useRoute } from 'vue-router';
import { Loader2 } from 'lucide-vue-next';
import ChatMessage from '../components/chat/ChatMessage.vue';
import InterruptCard from '../components/chat/InterruptCard.vue';
import ChatInput from '../components/chat/ChatInput.vue';
import { useChatStore } from '../stores/chat';
import { SessionStatus, ResumeAction, ApproveScope } from '../types/api';

const route = useRoute();
const chatStore = useChatStore();

const scrollAnchorRef = ref<HTMLDivElement | null>(null);

const sessionId = computed(() => (route.params.sessionId as string) || '');

// 会话状态呈现
const statusText = computed(() => {
  switch (chatStore.sessionStatus) {
    case SessionStatus.SS_RUNNING:
      return '处理/推导中...';
    case SessionStatus.SS_INTERRUPTED:
      return '已挂起 (等待人工审批)';
    case SessionStatus.SS_IDLE:
    default:
      return '就绪';
  }
});

const statusColorClass = computed(() => {
  switch (chatStore.sessionStatus) {
    case SessionStatus.SS_RUNNING:
      return 'bg-emerald-400 animate-ping';
    case SessionStatus.SS_INTERRUPTED:
      return 'bg-amber-400 animate-pulse';
    case SessionStatus.SS_IDLE:
    default:
      return 'bg-slate-500';
  }
});

// 监听路由变更或切换会话
watch(
  () => route.params.sessionId,
  (newId) => {
    if (newId) {
      chatStore.selectSession(newId as string);
    }
  },
  { immediate: true }
);

// 监听消息增加，自动滚动到底部
watch(
  () => chatStore.messages,
  () => {
    scrollToBottom();
  },
  { deep: true }
);

onMounted(() => {
  if (sessionId.value) {
    chatStore.selectSession(sessionId.value);
  }
  scrollToBottom();
});

function scrollToBottom() {
  nextTick(() => {
    if (scrollAnchorRef.value) {
      scrollAnchorRef.value.scrollIntoView({ behavior: 'smooth' });
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
