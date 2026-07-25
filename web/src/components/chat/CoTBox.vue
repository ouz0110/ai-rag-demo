<template>
  <div class="cot-box border border-indigo-500/20 bg-indigo-950/20 rounded-xl mb-3 overflow-hidden">
    <!-- 头部栏 (点击切换折叠) -->
    <div
      class="flex items-center justify-between px-3.5 py-2.5 cursor-pointer hover:bg-indigo-900/30 transition-colors select-none"
      @click="isExpanded = !isExpanded"
    >
      <div class="flex items-center gap-2 text-xs font-medium text-indigo-300">
        <!-- 思考中罗盘动画或完成图标 -->
        <span v-if="isStreaming" class="relative flex h-2 w-2">
          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-75"></span>
          <span class="relative inline-flex rounded-full h-2 w-2 bg-indigo-500"></span>
        </span>
        <Brain v-else :size="14" class="text-indigo-400" />
        <span>{{ isStreaming ? 'AI Agent 正在思考...' : '已深度思考推导' }}</span>
        <span class="text-indigo-400/60 font-mono">({{ reasoningText.length }} 字)</span>
      </div>

      <ChevronDown
        :size="14"
        class="text-indigo-400 transition-transform duration-200"
        :class="{ 'rotate-180': isExpanded }"
      />
    </div>

    <!-- 思考正文 -->
    <div
      v-show="isExpanded"
      class="px-4 py-3 text-xs text-indigo-200/80 leading-relaxed border-t border-indigo-500/10 font-mono whitespace-pre-wrap bg-black/20"
    >
      {{ reasoningText }}
      <span v-if="isStreaming" class="inline-block w-1.5 h-3 bg-indigo-400 animate-pulse ml-0.5"></span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { Brain, ChevronDown } from 'lucide-vue-next';

defineProps<{
  reasoningText: string;
  isStreaming?: boolean;
}>();

const isExpanded = ref(true);
</script>

<style scoped>
.cot-box {
  backdrop-filter: blur(8px);
}
</style>
