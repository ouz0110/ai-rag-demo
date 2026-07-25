<template>
  <div class="message-wrapper my-5 flex flex-col" :class="msg.role === 'user' ? 'items-end' : 'items-start'">
    <!-- 头部 Avatar & Agent 名称标签 -->
    <div class="flex items-center gap-2 mb-1.5 px-1 text-xs text-slate-400 select-none">
      <template v-if="msg.role === 'user'">
        <span>您</span>
        <div class="w-6 h-6 rounded-full bg-indigo-600 flex items-center justify-center text-white font-bold text-[10px]">
          U
        </div>
      </template>
      <template v-else>
        <div class="w-6 h-6 rounded-full bg-gradient-to-tr from-cyan-500 to-blue-600 flex items-center justify-center text-white font-bold text-[10px] shadow-sm">
          DS
        </div>
        <span class="font-semibold text-slate-200">DeepSeek Agent</span>
        <span v-if="msg.agent_name" class="px-1.5 py-0.2 rounded bg-slate-800 text-[10px] text-cyan-400 font-mono">
          @{{ msg.agent_name }}
        </span>
      </template>
    </div>

    <!-- 消息主体内容 -->
    <div
      class="max-w-[85%] md:max-w-[78%] rounded-2xl p-4 shadow-sm"
      :class="[
        msg.role === 'user'
          ? 'bg-indigo-600/90 text-white rounded-tr-none'
          : 'bg-[#1a1c23] border border-slate-800/80 text-slate-100 rounded-tl-none'
      ]"
    >
      <!-- 1. 助手专属: CoT 深度思考展开框 -->
      <CoTBox
        v-if="msg.role === 'assistant' && msg.reasoning_content"
        :reasoning-text="msg.reasoning_content"
        :is-streaming="msg.isStreaming && !msg.content"
      />

      <!-- 2. 助手专属: 自动工具调用列表 -->
      <template v-if="msg.role === 'assistant' && msg.tools && msg.tools.length > 0">
        <ToolCallBox v-for="t in msg.tools" :key="t.tool_call_id" :tool="t" />
      </template>

      <!-- 3. 用户/助手文本输出 -->
      <div v-if="msg.content" class="markdown-body" :class="{ 'cursor-blink': msg.isStreaming }" v-html="renderedContent"></div>

      <!-- 4. 报错提醒 -->
      <div v-if="msg.error" class="mt-2 p-2.5 bg-red-950/40 border border-red-800/50 rounded-lg text-xs text-red-300 flex items-center gap-2">
        <AlertCircle :size="14" />
        <span>{{ msg.error }}</span>
      </div>

      <!-- 快捷工具按钮 (复制等) -->
      <div v-if="msg.role === 'assistant' && msg.content && !msg.isStreaming" class="mt-3 pt-2 border-t border-slate-800/50 flex items-center justify-end gap-2 text-slate-400">
        <button
          @click="copyText"
          class="flex items-center gap-1 text-[11px] hover:text-slate-200 transition-colors"
          :title="copied ? '已复制' : '复制正文'"
        >
          <Check v-if="copied" :size="12" class="text-emerald-400" />
          <Copy v-else :size="12" />
          <span>{{ copied ? '已复制' : '复制' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { marked } from 'marked';
import hljs from 'highlight.js';
import { Copy, Check, AlertCircle } from 'lucide-vue-next';
import CoTBox from './CoTBox.vue';
import ToolCallBox from './ToolCallBox.vue';
import type { UIChatMessage } from '../../stores/chat';

const props = defineProps<{
  msg: UIChatMessage;
}>();

const copied = ref(false);

// marked 自定义语法高亮扩展配置
marked.use({
  renderer: {
    code({ text, lang }: { text: string; lang?: string }) {
      const validLang = lang && hljs.getLanguage(lang) ? lang : 'plaintext';
      const highlighted = hljs.highlight(text, { language: validLang }).value;
      return `<pre><code class="hljs language-${validLang}">${highlighted}</code></pre>`;
    },
  },
});

const renderedContent = computed(() => {
  if (!props.msg.content) return '';
  return marked.parse(props.msg.content) as string;
});

function copyText() {
  if (!props.msg.content) return;
  navigator.clipboard.writeText(props.msg.content);
  copied.value = true;
  setTimeout(() => {
    copied.value = false;
  }, 2000);
}
</script>
