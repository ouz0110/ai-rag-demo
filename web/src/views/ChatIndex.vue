<template>
  <div class="chat-index-page">
    <!-- 背景弥散光晕 -->
    <div class="bg-glow"></div>

    <!-- 中央欢迎与推荐提示区 -->
    <div class="welcome-container">
      <div class="hero-logo-box">
        <Bot :size="36" />
      </div>

      <h1 class="welcome-title">AI-RAG-DEMO</h1>
      <p class="welcome-desc">
        大模型 CoT 深度推理 · 流式打字机响应 · RAG 知识库检索 · 敏感 Tool 人工中断授权审批
      </p>

      <!-- 推荐 Prompt 卡片 -->
      <div class="prompts-grid">
        <div
          v-for="item in prompts"
          :key="item.title"
          @click="selectPrompt(item.content)"
          class="prompt-card"
        >
          <div class="card-top">
            <div class="card-icon">
              <component :is="item.icon" :size="16" />
            </div>
            <h3 class="card-title">{{ item.title }}</h3>
          </div>
          <p class="card-content">{{ item.content }}</p>
        </div>
      </div>
    </div>

    <!-- 底部常驻输入框 -->
    <ChatInput />
  </div>
</template>

<script setup lang="ts">
import { Bot, Code2, Cpu, FileSearch, HelpCircle } from 'lucide-vue-next';
import ChatInput from '../components/chat/ChatInput.vue';
import { useChatStore } from '../stores/chat';

const chatStore = useChatStore();

const prompts = [
  {
    title: '复杂代码重构与分析',
    icon: Code2,
    content: '分析后端数据逻辑与并发模型，帮我使用 Go/Vue3 优雅地重构...',
  },
  {
    title: 'Agent 工具自动化与流式审批',
    icon: Cpu,
    content: '调用后台 IoT/RAG 敏感工具并触发前端人工授权审批流程...',
  },
  {
    title: '深度逻辑思考与 CoT 推理',
    icon: FileSearch,
    content: '一步步推导为什么分布式一致性协议 Raft 能防止 split-brain 脑裂问题？',
  },
  {
    title: '系统架构设计咨询',
    icon: HelpCircle,
    content: '为高并发微服务架构设计缓存更新与分布式事务方案...',
  },
];

function selectPrompt(content: string) {
  chatStore.sendMessage(content);
}
</script>

<style scoped>
.chat-index-page {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  position: relative;
  overflow: hidden;
  background-color: #080a13;
}

.bg-glow {
  position: absolute;
  top: 35%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 450px;
  height: 450px;
  background: rgba(99, 102, 241, 0.12);
  border-radius: 50%;
  filter: blur(140px);
  pointer-events: none;
}

.welcome-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2rem 1.5rem;
  text-align: center;
  overflow-y: auto;
  z-index: 10;
  max-width: 800px;
  margin: 0 auto;
  width: 100%;
  box-sizing: border-box;
}

.hero-logo-box {
  width: 72px;
  height: 72px;
  border-radius: 22px;
  background: linear-gradient(135deg, #6366f1, #06b6d4);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  box-shadow: 0 10px 28px rgba(99, 102, 241, 0.35);
  margin-bottom: 1.25rem;
}

.welcome-title {
  font-size: 2.25rem;
  font-weight: 900;
  color: #f8fafc;
  letter-spacing: -0.02em;
  margin-bottom: 0.5rem;
}

.welcome-desc {
  font-size: 0.8125rem;
  color: #94a3b8;
  max-width: 520px;
  line-height: 1.6;
  margin-bottom: 2rem;
}

.prompts-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1rem;
  width: 100%;
}

@media (max-width: 640px) {
  .prompts-grid {
    grid-template-columns: 1fr;
  }
}

.prompt-card {
  padding: 1.125rem;
  background: rgba(15, 20, 36, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 18px;
  cursor: pointer;
  text-align: left;
  backdrop-filter: blur(12px);
  transition: all 0.25s ease;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.prompt-card:hover {
  background: rgba(20, 26, 48, 0.9);
  border-color: rgba(99, 102, 241, 0.4);
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3);
}

.card-top {
  display: flex;
  align-items: center;
  gap: 10px;
}

.card-icon {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  background: rgba(99, 102, 241, 0.15);
  color: #818cf8;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.2s ease;
}

.prompt-card:hover .card-icon {
  background: #6366f1;
  color: #ffffff;
}

.card-title {
  font-size: 0.8125rem;
  font-weight: 700;
  color: #f1f5f9;
  transition: color 0.2s ease;
}

.prompt-card:hover .card-title {
  color: #a5b4fc;
}

.card-content {
  font-size: 0.75rem;
  color: #94a3b8;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>

