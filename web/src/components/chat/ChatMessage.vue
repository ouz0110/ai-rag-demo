<template>
  <!-- 0. 上下文压缩卡片展示 -->
  <ContextCompressCard
    v-if="msg.role === 'system'"
    :compress-info="msg.compress_info"
    :text="msg.content"
  />

  <div
    v-else
    class="message-wrapper group relative"
    :class="[
      msg.role === 'user' ? 'user-layout' : 'assistant-layout',
      msg.agent_name && msg.agent_name !== 'main' ? 'sub-agent-wrapper ml-6 pl-3 border-l-2 border-amber-500/30' : ''
    ]"
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
      <div v-if="msg.role === 'assistant'" class="sender-info flex items-center gap-1.5">
        <span class="sender-name">AI-RAG Agent</span>
        <span
          v-if="!msg.agent_name || msg.agent_name === 'main'"
          class="agent-badge main-agent-badge"
          title="调度中心主 Agent"
        >
          🤖 主 Agent (@main)
        </span>
        <span
          v-else
          class="agent-badge sub-agent-badge"
          :title="`专有子 Agent: @${msg.agent_name}`"
        >
          ⚡ 子 Agent (@{{ msg.agent_name }})
        </span>
      </div>

      <!-- 消息 Card -->
      <div
        class="message-card"
        :class="[
          msg.role === 'user' ? 'user-card' : 'assistant-card',
          msg.agent_name && msg.agent_name !== 'main' ? 'sub-agent-card' : ''
        ]"
      >
        <!-- 1. 助手专属: CoT 深度思考展开框 -->
        <CoTBox
          v-if="msg.role === 'assistant' && msg.reasoning_content"
          :reasoning-text="msg.reasoning_content"
          :is-streaming="msg.isStreaming && (!msg.segments || msg.segments.length === 0)"
        />

        <!-- 2. 用户消息 (普通文本直接渲染) -->
        <div
          v-if="msg.role === 'user'"
          class="markdown-body"
          v-html="renderMarkdown(msg.content)"
        ></div>

        <!-- 3. 助手消息 (按 segments 顺序交织渲染小块: 文本 ➔ 工具 ➔ 文本 ➔ 工具) -->
        <div v-else class="segments-container flex flex-col gap-2">
          <!-- 子 Agent 专有折叠/展开控制栏 -->
          <div
            v-if="msg.agent_name && msg.agent_name !== 'main'"
            class="sub-agent-collapse-header flex items-center justify-between px-3 py-1.5 rounded-xl bg-amber-500/10 border border-amber-500/20 cursor-pointer select-none mb-1 hover:bg-amber-500/15 transition-colors"
            @click="isSubAgentExpanded = !isSubAgentExpanded"
          >
            <div class="flex items-center gap-2 text-xs font-semibold text-amber-300">
              <Bot :size="14" class="text-amber-400" />
              <span>子 Agent (@{{ msg.agent_name }}) 独立执行步骤明细</span>
            </div>
            <div class="flex items-center gap-1.5 text-[11px] text-amber-400/80">
              <span>{{ isSubAgentExpanded ? '点击折叠' : '点击展开查看' }}</span>
              <ChevronDown :size="13" class="transition-transform duration-200" :class="{ 'rotate-180': isSubAgentExpanded }" />
            </div>
          </div>

          <!-- 折叠卡片主体 (展开后具备固定最大高度与优雅滚动条) -->
          <div
            v-show="!msg.agent_name || msg.agent_name === 'main' || isSubAgentExpanded"
            :class="msg.agent_name && msg.agent_name !== 'main' ? 'sub-agent-body-scroll max-h-72 overflow-y-auto pr-1 space-y-2' : ''"
          >
            <!-- 助手等待/思考中 Loading 状态 (当尚未收到 content / reasoning / segments 时) -->
            <div
              v-if="msg.isStreaming && !msg.content && !msg.reasoning_content && (!msg.segments || msg.segments.length === 0)"
              class="agent-loading-state"
            >
              <Loader2 :size="16" class="animate-spin text-indigo-400" />
              <span class="loading-text font-medium">AI-RAG Agent 正在思考中...</span>
              <div class="loading-dots">
                <span class="dot"></span>
                <span class="dot"></span>
                <span class="dot"></span>
              </div>
            </div>

            <!-- 回退兼容: 如果 segments 为空，但 content 存在 -->
            <div
              v-if="(!msg.segments || msg.segments.length === 0) && msg.content"
              class="markdown-body"
              :class="{ 'cursor-blink': msg.isStreaming }"
              v-html="renderMarkdown(msg.content)"
            ></div>

            <!-- 按顺序动态渲染 segments 小块 -->
            <div
              v-for="(seg, idx) in msg.segments"
              :key="seg.id || idx"
              class="segment-item"
            >
              <!-- 文本小块 (独立区域 + 悬浮专属复制按钮) -->
              <div
                v-if="seg.type === 'text' && seg.content"
                class="text-segment-block group/segment relative"
              >
                <div
                  class="markdown-body"
                  :class="{ 'cursor-blink': msg.isStreaming && idx === msg.segments.length - 1 }"
                  v-html="renderMarkdown(seg.content)"
                ></div>

                <!-- 每个文本小块的悬浮操作按钮 -->
                <div class="segment-actions-bar">
                  <button
                    @click.stop="copySegmentText(seg.id, seg.content)"
                    class="action-btn"
                    :title="copiedSegmentId === seg.id ? '已复制小块内容' : '复制此小块'"
                  >
                    <Check v-if="copiedSegmentId === seg.id" :size="12" class="text-emerald-400" />
                    <Copy v-else :size="12" />
                    <span>{{ copiedSegmentId === seg.id ? '已复制' : '复制小块' }}</span>
                  </button>
                </div>
              </div>

              <!-- 工具调用小块 (独立卡片组件) -->
              <ToolCallBox
                v-else-if="seg.type === 'tool' && seg.tools && seg.tools.length > 0"
                :tools="seg.tools"
                class="my-1.5"
              />
            </div>
          </div>
        </div>

        <!-- 4. 报错提醒 -->
        <div v-if="msg.error" class="error-box mt-2">
          <AlertCircle :size="15" class="error-icon" />
          <span>{{ msg.error }}</span>
        </div>
      </div>

      <!-- 5. 悬浮操作栏 (整条消息大块复制按钮) -->
      <div class="hover-actions-bar" v-if="msg.content || msg.reasoning_content">
        <!-- 复制整条大块正文按钮 -->
        <button
          v-if="msg.content"
          @click="copyText"
          class="action-btn"
          :title="copied ? '已复制全部正文' : '复制全部正文'"
        >
          <Check v-if="copied" :size="13" class="text-emerald-400" />
          <Copy v-else :size="13" />
          <span>{{ copied ? '已复制全部' : '复制全部' }}</span>
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

        <!-- 🎯 继续生成按钮 (仅当当前会话处于 SS_PAUSED 暂停状态且为全局最后一条消息时展示) -->
        <button
          v-if="canContinueMessage"
          @click="chatStore.continueGeneration()"
          class="action-btn text-amber-300 border-amber-500/30 hover:bg-amber-500/10"
          title="顺着当前回答继续生成后续内容"
        >
          <Play :size="13" class="text-amber-400 fill-amber-400/20" />
          <span>继续生成</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { marked } from 'marked';
import hljs from 'highlight.js';
import { Bot, Copy, Check, Brain, AlertCircle, Loader2, ChevronDown, Play } from 'lucide-vue-next';
import CoTBox from './CoTBox.vue';
import ToolCallBox from './ToolCallBox.vue';
import ContextCompressCard from './ContextCompressCard.vue';
import { useUserStore } from '../../stores/user';
import { useChatStore, type UIChatMessage } from '../../stores/chat';
import { SessionStatus } from '../../types/api';

const props = defineProps<{
  msg: UIChatMessage;
}>();

const userStore = useUserStore();
const chatStore = useChatStore();
const copied = ref(false);
const copiedReasoning = ref(false);
const copiedSegmentId = ref<string>('');
const isSubAgentExpanded = ref(true);

const isVeryLastMessageInChat = computed(() => {
  if (props.msg.role !== 'assistant') return false;
  const list = chatStore.messages;
  if (list.length === 0) return false;
  return list[list.length - 1].id === props.msg.id;
});

const canContinueMessage = computed(() => {
  return !chatStore.isGenerating && chatStore.sessionStatus === SessionStatus.SS_PAUSED && isVeryLastMessageInChat.value;
});

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

function renderMarkdown(content: string) {
  if (!content) return '';
  return marked.parse(content) as string;
}

// 复制某个小块文本
function copySegmentText(segmentId: string, content: string) {
  if (!content) return;
  navigator.clipboard.writeText(content);
  copiedSegmentId.value = segmentId;
  setTimeout(() => {
    copiedSegmentId.value = '';
  }, 2000);
}

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

.agent-badge {
  padding: 1.5px 7px;
  border-radius: 6px;
  font-size: 0.65rem;
  font-weight: 600;
  font-family: monospace;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.main-agent-badge {
  background: rgba(99, 102, 241, 0.15);
  border: 1px solid rgba(99, 102, 241, 0.35);
  color: #a5b4fc;
}

.sub-agent-badge {
  background: rgba(245, 158, 11, 0.15);
  border: 1px solid rgba(245, 158, 11, 0.35);
  color: #fbbf24;
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

.sub-agent-card {
  background: rgba(26, 20, 38, 0.85) !important;
  border: 1px solid rgba(245, 158, 11, 0.25) !important;
  box-shadow: 0 4px 16px rgba(245, 158, 11, 0.08) !important;
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

.text-segment-block {
  position: relative;
}

.segment-actions-bar {
  position: absolute;
  top: 4px;
  right: 6px;
  opacity: 0;
  visibility: hidden;
  transition: opacity 0.2s ease, visibility 0.2s ease;
  z-index: 10;
}

.segment-actions-bar .action-btn {
  background: rgba(15, 23, 42, 0.85);
  border: 1px solid rgba(255, 255, 255, 0.12);
  backdrop-filter: blur(8px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
  padding: 3px 8px;
  border-radius: 6px;
}

.segment-actions-bar .action-btn:hover {
  background: rgba(99, 102, 241, 0.25);
  border-color: rgba(99, 102, 241, 0.4);
  color: #a5b4fc;
}

.text-segment-block:hover .segment-actions-bar {
  opacity: 1;
  visibility: visible;
}

/* Agent 思考 / 等待响应 Loading 样式 */
.agent-loading-state {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0.375rem 0.25rem;
  color: #a5b4fc;
  font-size: 0.84375rem;
  user-select: none;
}

.loading-text {
  background: linear-gradient(90deg, #a5b4fc 0%, #c084fc 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.loading-dots {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: 2px;
}

.loading-dots .dot {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background-color: #818cf8;
  animation: dotPulse 1.4s infinite ease-in-out both;
}

.loading-dots .dot:nth-child(1) {
  animation-delay: -0.32s;
}

.loading-dots .dot:nth-child(2) {
  animation-delay: -0.16s;
}

@keyframes dotPulse {
  0%,
  80%,
  100% {
    transform: scale(0.6);
    opacity: 0.4;
  }
  40% {
    transform: scale(1.2);
    opacity: 1;
    background-color: #c084fc;
  }
}
</style>

