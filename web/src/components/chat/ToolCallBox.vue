<template>
  <div class="tool-call-box border border-slate-800 bg-slate-900/60 rounded-lg mb-3 p-2.5 text-xs font-mono">
    <div class="flex items-center justify-between cursor-pointer" @click="isOpen = !isOpen">
      <div class="flex items-center gap-2">
        <Wrench v-if="tool.status === 'completed'" :size="13" class="text-emerald-400" />
        <Loader2 v-else :size="13" class="text-sky-400 animate-spin" />
        <span class="text-slate-300 font-semibold">调用工具: {{ tool.tool_name }}</span>
        <span
          class="px-1.5 py-0.5 rounded text-[10px]"
          :class="tool.status === 'completed' ? 'bg-emerald-950 text-emerald-400 border border-emerald-800/40' : 'bg-sky-950 text-sky-400 border border-sky-800/40'"
        >
          {{ tool.status === 'completed' ? '成功' : '执行中...' }}
        </span>
      </div>
      <ChevronDown :size="13" class="text-slate-400 transition-transform duration-200" :class="{ 'rotate-180': isOpen }" />
    </div>

    <!-- 展开详情 -->
    <div v-show="isOpen" class="mt-2 pt-2 border-t border-slate-800/80 space-y-1.5">
      <div>
        <span class="text-slate-400">调用参数:</span>
        <pre class="mt-0.5 p-1.5 bg-black/40 rounded text-slate-300 overflow-x-auto text-[11px]">{{ formatJson(tool.arguments) }}</pre>
      </div>
      <div v-if="tool.result_preview">
        <span class="text-slate-400">结果预览:</span>
        <pre class="mt-0.5 p-1.5 bg-black/40 rounded text-emerald-300/90 overflow-x-auto text-[11px]">{{ tool.result_preview }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { Wrench, Loader2, ChevronDown } from 'lucide-vue-next';
import type { UIStreamTool } from '../../stores/chat';

defineProps<{
  tool: UIStreamTool;
}>();

const isOpen = ref(false);

function formatJson(jsonStr: string) {
  try {
    const obj = JSON.parse(jsonStr);
    return JSON.stringify(obj, null, 2);
  } catch (e) {
    return jsonStr;
  }
}
</script>
