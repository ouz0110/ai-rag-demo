<template>
  <div
    class="message-wrapper group relative"
    :class="msg.role === 'user' ? 'user-layout' : 'assistant-layout'"
    @click="handleCardClick"
  >
    <!-- Avatar 头像栏 -->
    <div class="avatar-box">
      <div v-if="msg.role === 'user'" class="user-avatar">
        {{ userAvatarText }}
      </div>
      <div v-else class="bot-avatar">
        <Bot :size="16" />
      </div>
    </div>

    <!-- 消息正文与卡片区 -->
    <div class="message-content-wrapper">
      <!-- 头部 Sender 身份信息 (仅助手展示) -->
      <div v-if="msg.role === 'assistant'" class="sender-info">
        <span class="sender-name">AI-RAG Agent</span>
        <span v-if="msg.agent_name" class="agent-tag">@{{ msg.agent_name }}</span>
      </div>

      <!-- 消息 Card -->
      <div
        class="message-card"
        :class="msg.role === 'user' ? 'user-card' : 'assistant-card'"
      >
        <!-- 1. 助手专属: CoT 深度思考展开框 -->
        <CoTBox
          v-if="msg.role === 'assistant' && msg.reasoning_content"
          :reasoning-text="msg.reasoning_content"
          :is-streaming="msg.isStreaming && !msg.content"
        />

        <!-- 2. 用户/助手文本输出 -->
        <div
          v-if="msg.content"
          class="markdown-body"
          :class="{ 'cursor-blink': msg.isStreaming }"
          v-html="renderedContent"
        ></div>

        <!-- 3. 助手专属: 自动工具调用组合卡片 (位于文本下方，逻辑更连贯) -->
        <ToolCallBox
          v-if="msg.role === 'assistant' && msg.tools && msg.tools.length > 0"
          :tools="msg.tools"
        />

        <!-- 4. 报错提醒 -->
        <div v-if="msg.error" class="error-box">
          <AlertCircle :size="15" class="error-icon" />
          <span>{{ msg.error }}</span>
        </div>
      </div>

      <!-- 5. 悬浮操作栏 (主流 AI Chat 模式: 悬浮 Message 时才展示复制按钮) -->
      <div class="hover-actions-bar" v-if="msg.content || msg.reasoning_content">
        <!-- 复制正文按钮 -->
        <button
          v-if="msg.content"
          @click="copyText"
          class="action-btn"
          :title="copied ? '已复制正文' : '复制正文'"
        >
          <Check v-if="copied" :size="13" class="text-emerald-400" />
          <Copy v-else :size="13" />
          <span>{{ copied ? '已复制' : '复制' }}</span>
        </button>

        <!-- 复制 CoT 思考过程按钮 -->
        <button
          v-if="msg.role === 'assistant' && msg.reasoning_content"
          @click="copyReasoning"
          class="action-btn"
          :title="copiedReasoning ? '已复制思考历程' : '复制思考历程'"
        >
          <Check v-if="copiedReasoning" :size="13" class="text-emerald-400" />
          <Brain v-else :size="13" />
          <span>{{ copiedReasoning ? '已复制思考' : '复制思考' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { marked } from 'marked';
import hljs from 'highlight.js';
import { Bot, Copy, Check, Brain, AlertCircle } from 'lucide-vue-next';
import CoTBox from './CoTBox.vue';
import ToolCallBox from './ToolCallBox.vue';
import { useUserStore } from '../../stores/user';
import type { UIChatMessage } from '../../stores/chat';

const props = defineProps<{
  msg: UIChatMessage;
}>();

const userStore = useUserStore();
const copied = ref(false);
const copiedReasoning = ref(false);

const userAvatarText = computed(() => {
  const name = userStore.userInfo?.nickname || userStore.userInfo?.account || 'U';
  return name[0].toUpperCase();
});

// 自定义 marked 代码块渲染器，植入顶部代码头栏与复制代码按钮
marked.use({
  renderer: {
    code({ text, lang }: { text: string; lang?: string }) {
      const validLang = lang && hljs.getLanguage(lang) ? lang : 'plaintext';
      const highlighted = hljs.highlight(text, { language: validLang }).value;
      const displayLang = validLang.toLowerCase();
      const encodedCode = encodeURIComponent(text);

      return `
        <div class="code-block-wrapper">
          <div class="code-block-header">
            <span class="code-lang">${displayLang}</span>
            <button class="code-copy-btn" data-code="${encodedCode}" type="button">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
              <span>复制代码</span>
            </button>
          </div>
          <pre><code class="hljs language-${validLang}">${highlighted}</code></pre>
        </div>
      `;
    },
  },
});

const renderedContent = computed(() => {
  if (!props.msg.content) return '';
  return marked.parse(props.msg.content) as string;
});

// 复制消息正文
function copyText() {
  if (!props.msg.content) return;
  navigator.clipboard.writeText(props.msg.content);
  copied.value = true;
  setTimeout(() => {
    copied.value = false;
  }, 2000);
}

// 复制思考过程
function copyReasoning() {
  if (!props.msg.reasoning_content) return;
  navigator.clipboard.writeText(props.msg.reasoning_content);
  copiedReasoning.value = true;
  setTimeout(() => {
    copiedReasoning.value = false;
  }, 2000);
}

// 代码块内部复制代码按钮事件委托
function handleCardClick(e: MouseEvent) {
  const btn = (e.target as HTMLElement).closest('.code-copy-btn') as HTMLElement | null;
  if (!btn) return;

  const rawCode = btn.getAttribute('data-code');
  if (rawCode) {
    const text = decodeURIComponent(rawCode);
    navigator.clipboard.writeText(text);

    const span = btn.querySelector('span');
    if (span) {
      const originalText = span.innerText;
      span.innerText = '已复制!';
      setTimeout(() => {
        span.innerText = originalText;
      }, 2000);
    }
  }
}
</script>

<style scoped>
.message-wrapper {
  display: flex;
  gap: 12px;
  margin-bottom: 1.5rem;
  width: 100%;
}

.user-layout {
  flex-direction: row-reverse;
}

.assistant-layout {
  flex-direction: row;
}

.avatar-box {
  flex-shrink: 0;
  padding-top: 2px;
}

.user-avatar {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: linear-gradient(135deg, #6366f1, #4f46e5);
  color: #ffffff;
  font-weight: 800;
  font-size: 0.8125rem;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 8px rgba(99, 102, 241, 0.3);
}

.bot-avatar {
  width: 34px;
  height: 34px;
  border-radius: 12px;
  background: linear-gradient(135deg, #6366f1, #06b6d4);
  color: #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.35);
}

.message-content-wrapper {
  display: flex;
  flex-direction: column;
  max-width: 86%;
}

.user-layout .message-content-wrapper {
  align-items: flex-end;
}

.assistant-layout .message-content-wrapper {
  align-items: flex-start;
}

.sender-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  padding-left: 2px;
}

.sender-name {
  font-size: 0.78125rem;
  font-weight: 700;
  color: #f1f5f9;
}

.agent-tag {
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(99, 102, 241, 0.15);
  border: 1px solid rgba(99, 102, 241, 0.25);
  color: #a5b4fc;
  font-size: 0.65rem;
  font-family: monospace;
}

.message-card {
  border-radius: 18px;
  padding: 0.875rem 1.125rem;
  font-size: 0.9375rem;
  line-height: 1.6;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
  word-break: break-word;
}

.user-card {
  background: linear-gradient(135deg, #4f46e5 0%, #4338ca 100%);
  color: #ffffff;
  border-top-right-radius: 4px;
}

.assistant-card {
  background: rgba(15, 20, 36, 0.75);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #f8fafc;
  border-top-left-radius: 4px;
  backdrop-filter: blur(12px);
}

.error-box {
  margin-top: 8px;
  padding: 8px 12px;
  background: rgba(127, 29, 29, 0.3);
  border: 1px solid rgba(239, 68, 68, 0.4);
  border-radius: 8px;
  font-size: 0.75rem;
  color: #fca5a5;
  display: flex;
  align-items: center;
  gap: 8px;
}

.error-icon {
  color: #f87171;
  flex-shrink: 0;
}

/* 主流 悬浮操作栏 (Hover 展示) */
.hover-actions-bar {
  opacity: 0;
  visibility: hidden;
  transition: opacity 0.2s ease, visibility 0.2s ease;
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  padding: 2px;
}

.message-wrapper:hover .hover-actions-bar {
  opacity: 1;
  visibility: visible;
}

.action-btn {
  background: rgba(30, 41, 59, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 4px 10px;
  color: #94a3b8;
  font-size: 0.725rem;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: all 0.2s ease;
}

.action-btn:hover {
  background: rgba(99, 102, 241, 0.15);
  border-color: rgba(99, 102, 241, 0.3);
  color: #a5b4fc;
}
</style>

