<template>
  <div class="chat-input-wrapper max-w-4xl mx-auto w-full px-4 pb-4">
    <div
      class="glass-panel p-3 rounded-2xl shadow-2xl border border-slate-700/60 focus-within:border-indigo-500/80 transition-all duration-300 relative"
    >
      <!-- 提示语/模型指示 -->
      <div class="flex items-center justify-between pb-2 px-1 text-xs text-slate-400 select-none">
        <div class="flex items-center gap-1.5">
          <Sparkles :size="13" class="text-indigo-400" />
          <span class="font-medium text-slate-300">DeepSeek-R1 (流式推理模式)</span>
        </div>
        <span class="text-[11px] text-slate-500">Shift + Enter 换行 | Enter 发送</span>
      </div>

      <!-- 输入框 TextArea -->
      <textarea
        v-model="inputContent"
        rows="2"
        placeholder="给 DeepSeek 发送消息 (支持 Markdown, 代码分析, 工具指令)..."
        class="w-full bg-transparent resize-none border-none text-sm text-slate-100 placeholder:text-slate-500 focus:outline-none leading-relaxed"
        @keydown.enter.exact.prevent="handleSend"
      ></textarea>

      <!-- 底部控制栏 -->
      <div class="flex items-center justify-between pt-2 border-t border-slate-800/60">
        <div class="flex items-center gap-2 text-xs text-slate-400">
          <button
            @click="chatStore.resetCurrentChat"
            class="px-2 py-1 hover:bg-slate-800 rounded flex items-center gap-1 transition-colors"
            title="开启新会话"
          >
            <Plus :size="13" />
            <span>新对话</span>
          </button>
        </div>

        <!-- 发送/中断按钮 -->
        <button
          v-if="chatStore.isGenerating"
          @click="chatStore.stopGeneration"
          class="px-3 py-1.5 bg-red-600 hover:bg-red-500 text-white rounded-lg text-xs font-semibold flex items-center gap-1.5 shadow-md transition-all animate-pulse"
        >
          <Square :size="12" class="fill-current" />
          <span>停止生成</span>
        </button>

        <button
          v-else
          @click="handleSend"
          :disabled="!inputContent.trim()"
          class="px-4 py-1.5 btn-primary text-xs font-semibold rounded-lg flex items-center gap-1.5 disabled:opacity-40 disabled:cursor-not-allowed disabled:transform-none"
        >
          <span>发送</span>
          <Send :size="13" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { Sparkles, Plus, Send, Square } from 'lucide-vue-next';
import { useChatStore } from '../../stores/chat';

const chatStore = useChatStore();
const inputContent = ref('');

function handleSend() {
  const text = inputContent.value.trim();
  if (!text || chatStore.isGenerating) return;

  chatStore.sendMessage(text);
  inputContent.value = '';
}
</script>
