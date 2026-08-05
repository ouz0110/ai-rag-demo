<template>
  <div class="chat-input-container">
    <div class="input-card">
      <!-- 顶栏：指示图标、继续生成按钮与按键快捷提示 -->
      <div class="card-header">
        <div class="brand-tag">
          <Sparkles :size="13" class="tag-icon" />
          <span>AI-RAG 大模型流式对话平台</span>
        </div>

        <div class="header-right">
          <!-- 🎯 固定在输入框右上角的【继续生成】按钮 -->
          <button
            v-if="canContinue"
            @click="chatStore.continueGeneration()"
            class="continue-header-btn"
            title="顺着上一条回答继续接续生成"
          >
            <Play :size="13" class="fill-amber-400/20 text-amber-400" />
            <span>继续生成</span>
          </button>

          <div class="shortcut-hint">
            <span class="hint-key">Shift + Enter</span> 换行 <span class="divider">|</span> <span class="hint-key">Enter</span> 发送
          </div>
        </div>
      </div>

      <!-- 输入框 TextArea -->
      <div class="textarea-wrapper">
        <textarea
          v-model="inputContent"
          rows="2"
          placeholder="给 AI-RAG Agent 发送消息... (支持 Markdown 提问、代码重构、RAG 向量检索与敏感工具授权处理)"
          class="chat-textarea"
          @keydown.enter.exact.prevent="handleSend"
        ></textarea>
      </div>

      <!-- 底部工具与发送控制栏 -->
      <div class="card-footer">
        <div class="flex items-center gap-3">
          <KBSelector />
          <div class="sse-status">
            <span class="status-dot"></span>
            <span>SSE 流式响应就绪</span>
          </div>
        </div>

        <div class="action-buttons">
          <button
            v-if="chatStore.isGenerating"
            @click="chatStore.stopGeneration()"
            class="stop-btn"
          >
            <Square :size="14" class="fill-current" />
            <span>停止生成</span>
          </button>

          <button
            v-else
            @click="handleSend"
            :disabled="!inputContent.trim()"
            class="send-btn"
          >
            <span>发送消息</span>
            <Send :size="14" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { Sparkles, Send, Square, Play } from 'lucide-vue-next';
import { useChatStore } from '../../stores/chat';
import { SessionStatus } from '../../types/api';
import KBSelector from '../KBSelector.vue';

const chatStore = useChatStore();
const inputContent = ref('');

const canContinue = computed(() => {
  return !chatStore.isGenerating && chatStore.sessionStatus === SessionStatus.SS_PAUSED;
});

function handleSend() {
  const text = inputContent.value.trim();
  if (!text || chatStore.isGenerating) return;

  chatStore.sendMessage(text);
  inputContent.value = '';
}
</script>

<style scoped>
.chat-input-container {
  width: 100%;
  max-width: 860px;
  margin: 0 auto;
  padding: 0 1.5rem 1.25rem 1.5rem;
  flex-shrink: 0;
  box-sizing: border-box;
  z-index: 20;
}

.input-card {
  background-color: rgba(15, 19, 34, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 20px;
  padding: 0.875rem 1.125rem;
  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(20px);
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.input-card:focus-within {
  border-color: rgba(99, 102, 241, 0.8);
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.2), 0 16px 36px rgba(0, 0, 0, 0.45);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  font-size: 0.75rem;
}

.brand-tag {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #cbd5e1;
  font-weight: 600;
}

.tag-icon {
  color: #818cf8;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.continue-header-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 10px;
  font-size: 11.5px;
  font-weight: 600;
  color: #fcd34d;
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.35);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: 0 0 10px rgba(245, 158, 11, 0.15);
}

.continue-header-btn:hover {
  background: rgba(245, 158, 11, 0.25);
  border-color: rgba(245, 158, 11, 0.6);
  transform: translateY(-1px);
  box-shadow: 0 2px 12px rgba(245, 158, 11, 0.3);
}

.continue-header-btn:active {
  transform: translateY(0);
}

.shortcut-hint {
  color: #64748b;
  font-size: 0.7rem;
  display: flex;
  align-items: center;
  gap: 4px;
}

.hint-key {
  color: #94a3b8;
  font-weight: 600;
  font-family: monospace;
}

.divider {
  color: #334155;
  margin: 0 2px;
}

.textarea-wrapper {
  width: 100%;
  padding: 0.25rem 0;
}

.chat-textarea {
  width: 100% !important;
  min-height: 60px !important;
  max-height: 160px !important;
  background: transparent !important;
  border: none !important;
  outline: none !important;
  color: #f8fafc !important;
  font-size: 0.875rem !important;
  line-height: 1.6 !important;
  resize: none !important;
  box-sizing: border-box !important;
  font-family: inherit !important;
}

.chat-textarea::placeholder {
  color: #475569 !important;
}

.card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-top: 0.5rem;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.sse-status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.7rem;
  color: #64748b;
  font-family: monospace;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background-color: #34d399;
  box-shadow: 0 0 6px #34d399;
}

.action-buttons {
  display: flex;
  align-items: center;
  gap: 10px;
}

.send-btn {
  height: 36px;
  padding: 0 1.25rem;
  background: linear-gradient(135deg, #6366f1 0%, #4f46e5 100%);
  border: none;
  border-radius: 12px;
  color: #ffffff;
  font-size: 0.78125rem;
  font-weight: 700;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
  transition: all 0.2s ease;
}

.send-btn:hover:not(:disabled) {
  background: linear-gradient(135deg, #818cf8 0%, #6366f1 100%);
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(99, 102, 241, 0.45);
}

.send-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  box-shadow: none;
}

.stop-btn {
  height: 36px;
  padding: 0 1.25rem;
  background: #dc2626;
  border: none;
  border-radius: 12px;
  color: #ffffff;
  font-size: 0.78125rem;
  font-weight: 700;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  box-shadow: 0 4px 12px rgba(220, 38, 38, 0.4);
  animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}

.stop-btn:hover {
  background: #ef4444;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.8; }
}
</style>

