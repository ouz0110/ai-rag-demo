<template>
  <div class="compress-card-container my-4 flex flex-col items-center w-full">
    <!-- 分割线样式 -->
    <div class="divider-line w-full flex items-center justify-center gap-3">
      <div class="h-px bg-gradient-to-r from-transparent via-indigo-500/30 to-transparent flex-1"></div>
      
      <!-- 主内容小卡片 -->
      <div class="compress-badge flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-slate-900/80 border border-indigo-500/30 backdrop-blur-md shadow-lg shadow-indigo-950/40 text-xs text-indigo-200">
        <Loader2 v-if="isCompressing" class="w-3.5 h-3.5 text-indigo-400 animate-spin" />
        <Sparkles v-else class="w-3.5 h-3.5 text-indigo-400 animate-pulse" />
        <span class="font-medium">
          {{ isCompressing ? '正在提炼历史记忆并压缩上下文...' : '上下文已自动压缩提炼' }}
        </span>
        <span v-if="!isCompressing && compressInfo?.original_tokens && compressInfo?.compressed_tokens" class="tokens-tag px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-300 font-mono text-[10px]">
          {{ compressInfo.original_tokens }} → {{ compressInfo.compressed_tokens }} Tokens
        </span>
        <button 
          v-if="summaryText && !isCompressing" 
          @click="isExpanded = !isExpanded" 
          class="ml-1 hover:text-white transition-colors flex items-center gap-0.5 text-[11px] text-indigo-300 underline underline-offset-2"
        >
          <span>{{ isExpanded ? '收起摘要' : '查看摘要' }}</span>
          <ChevronDown class="w-3 h-3 transition-transform duration-200" :class="{ 'rotate-180': isExpanded }" />
        </button>
      </div>

      <div class="h-px bg-gradient-to-r from-transparent via-indigo-500/30 to-transparent flex-1"></div>
    </div>

    <!-- 展开的摘要预览与熔断提醒面板 -->
    <Transition
      enter-active-class="transition-all duration-300 ease-out"
      enter-from-class="opacity-0 -translate-y-2 max-h-0"
      enter-to-class="opacity-100 translate-y-0 max-h-96"
      leave-active-class="transition-all duration-200 ease-in"
      leave-from-class="opacity-100 translate-y-0 max-h-96"
      leave-to-class="opacity-0 -translate-y-2 max-h-0"
    >
      <div v-if="isExpanded && !isCompressing" class="summary-details-panel mt-2.5 w-full max-w-2xl bg-slate-900/90 border border-indigo-500/20 rounded-xl p-3.5 text-xs text-slate-300 shadow-xl backdrop-blur-md">
        <!-- 熔断提醒 -->
        <div v-if="compressInfo?.is_max_limit_reached" class="flex items-center justify-between gap-3 p-2.5 mb-2 bg-amber-500/10 border border-amber-500/30 rounded-lg text-amber-300 text-xs">
          <div class="flex items-center gap-2">
            <AlertTriangle class="w-4 h-4 text-amber-400 shrink-0" />
            <span>已达到单会话最大压缩次数上限，系统已启动滚动滑动窗口运行。建议开辟新对话获取最精准回答。</span>
          </div>
          <button 
            @click="handleNewChat" 
            class="px-2.5 py-1 bg-amber-500/20 hover:bg-amber-500/30 border border-amber-500/40 rounded text-amber-200 text-xs font-medium whitespace-nowrap transition-colors"
          >
            开启新会话
          </button>
        </div>

        <!-- 摘要正文 -->
        <div v-if="summaryText" class="summary-content font-sans leading-relaxed text-slate-300 whitespace-pre-wrap">
          {{ summaryText }}
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { Sparkles, ChevronDown, AlertTriangle, Loader2 } from 'lucide-vue-next';
import { useChatStore } from '../../stores/chat';
import { CompressStatus, type CompressInfo } from '../../types/api';

const props = defineProps<{
  compressInfo?: CompressInfo;
  text?: string;
}>();

const chatStore = useChatStore();
const isExpanded = ref(false);

const isCompressing = computed(() => {
  const status = props.compressInfo?.status;
  // 1. 显式根据 status 枚举判定 (1: 压缩中, 2: 已完成)
  if (status === CompressStatus.CS_COMPRESSING) {
    return true;
  }
  if (status === CompressStatus.CS_COMPLETED) {
    return false;
  }

  // 2. 降级兼容: 根据 tokens 数据与占位符文案判定
  if (
    props.compressInfo?.original_tokens &&
    !props.compressInfo?.compressed_tokens &&
    !props.compressInfo?.is_max_limit_reached
  ) {
    return true;
  }
  if (summaryText.value === '正在提炼历史记忆摘要...') {
    return true;
  }
  return false;
});

const summaryText = computed(() => {
  if (props.compressInfo?.summary_preview) {
    return props.compressInfo.summary_preview;
  }
  return props.text || '';
});

function handleNewChat() {
  chatStore.resetCurrentChat();
}
</script>

<style scoped>
.compress-card-container {
  user-select: none;
}
.summary-content {
  max-height: 220px;
  overflow-y: auto;
}
</style>
