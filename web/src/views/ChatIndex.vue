<template>
  <div class="chat-index-page">
    <!-- 背景弥散光晕 -->
    <div class="bg-glow glow-primary"></div>
    <div class="bg-glow glow-secondary"></div>
    <div class="bg-glow glow-accent"></div>

    <!-- 中央欢迎与技术亮点展示区 (绝对居中) -->
    <div class="welcome-container">
      <div class="welcome-content">
        <!-- 顶部 Badge 与 Logo -->
        <div class="hero-header">
          <div class="logo-wrapper">
            <div class="logo-glow-ring"></div>
            <div class="hero-logo-box">
              <Bot :size="36" class="logo-icon" />
            </div>
          </div>
          <div class="hero-badge">
            <Sparkles :size="12" class="text-amber-400 animate-pulse" />
            <span>Enterprise Multi-Agent Platform</span>
          </div>
        </div>

        <h1 class="welcome-title">AI-RAG Agent 核心架构引擎</h1>

        <!-- 开源项目地址 显目标签 -->
        <a
          href="https://github.com/ouz0110/ai-rag-demo"
          target="_blank"
          rel="noopener noreferrer"
          class="github-hero-badge"
          title="点击前往 GitHub 开源项目"
        >
          <Github :size="15" class="badge-icon-gh" />
          <span class="badge-text">开源架构仓库: <strong class="badge-repo">github.com/ouz0110/ai-rag-demo</strong></span>
          <ExternalLink :size="12" class="badge-ext" />
        </a>

        <p class="welcome-desc">
          基于 <strong>Go-Kratos + Vue3</strong> 的企业级 Agent 平台 · 支持多 Agent 动态编排、秒级 Checkpoint 断点续跑、高危指令人机审批与 RAG 向量检索
        </p>

        <!-- 核心技术亮点卡片自适应网格 -->
        <div class="prompts-grid">
          <div
            v-for="item in featurePrompts"
            :key="item.title"
            @click="selectPrompt(item.prompt)"
            class="prompt-card"
          >
            <!-- 顶部高光边框条 -->
            <div class="card-glow-line" :class="item.colorClass"></div>

            <div class="card-header-row">
              <div class="card-icon-box" :class="item.colorClass">
                <component :is="item.icon" :size="18" />
              </div>
              <span class="tech-pill">{{ item.tag }}</span>
            </div>

            <h3 class="card-title">{{ item.title }}</h3>
            <p class="card-content">{{ item.description }}</p>

            <div class="card-footer-action">
              <span class="action-text">点击体验该核心技术</span>
              <span class="action-arrow">→</span>
            </div>
          </div>
        </div>

        <!-- 底部技术栈亮点横幅 -->
        <div class="tech-stack-bar">
          <span class="tech-tag"><span class="dot green"></span> Go / Go-Kratos</span>
          <span class="tech-tag"><span class="dot blue"></span> ReAct Agent 架构</span>
          <span class="tech-tag"><span class="dot purple"></span> Checkpoint 断点快照</span>
          <span class="tech-tag"><span class="dot amber"></span> 人机交互授权审批</span>
          <span class="tech-tag"><span class="dot cyan"></span> SSE 打字机流式传输</span>
        </div>
      </div>
    </div>

    <!-- 底部常驻输入框 -->
    <ChatInput />
  </div>
</template>

<script setup lang="ts">
import { Bot, Cpu, ShieldCheck, Database, Zap, Sparkles, Github, ExternalLink } from 'lucide-vue-next';
import ChatInput from '../components/chat/ChatInput.vue';
import { useChatStore } from '../stores/chat';

const chatStore = useChatStore();

const featurePrompts = [
  {
    title: '多 Agent 动态编排与 Checkpoint 续跑',
    tag: 'Multi-Agent & Checkpoint',
    icon: Cpu,
    colorClass: 'theme-indigo',
    description: '自动路由委派 file_analyzer 等子 Agent 处理复杂代码，支持中途中断后的服务端快照归档与秒级断点无缝续接。',
    prompt: '请委派 file_analyzer 子 Agent 帮我分析当前项目的核心架构设计，并给出重构优化建议。',
  },
  {
    title: 'ReAct 工具链与高危指令人机审批',
    tag: 'Human-in-the-Loop Approval',
    icon: ShieldCheck,
    colorClass: 'theme-amber',
    description: '调度 terminal 物理终端与文件系统，遇到危险修改性指令自动阻断并弹出人机审批窗口，经授权后平滑接续。',
    prompt: '请使用 terminal 工具查看当前项目根目录的文件列表，并分析系统安全防护机制。',
  },
  {
    title: '多租户 RAG 向量知识库精准检索',
    tag: 'Hybrid RAG Knowledge Engine',
    icon: Database,
    colorClass: 'theme-cyan',
    description: '结合多租户向量数据库引擎与 Embedding 召回，智能检索专有业务文档与架构规范，实现毫秒级精准问答。',
    prompt: '开启 RAG 检索知识库，帮我查询我们项目的核心架构规范与开发禁令。',
  },
  {
    title: 'SSE 全双工流式打字与 CoT 逻辑推导',
    tag: 'SSE Streaming & CoT Reasoning',
    icon: Zap,
    colorClass: 'theme-purple',
    description: '基于全双工 SSE 流式打字传输，配合 CoT 链式推导与长会话上下文动态压缩引擎，极致降低 Token 消耗。',
    prompt: '使用一步步推导的 CoT 思维链，详细分析分布式一致性协议 Raft 如何解决 split-brain 脑裂问题？',
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
  background-color: #060812;
  background-image: radial-gradient(circle at 50% 0%, #0d1226 0%, #060812 75%);
}

/* 弥散多重光晕背景 */
.bg-glow {
  position: absolute;
  border-radius: 50%;
  pointer-events: none;
  filter: blur(140px);
}

.glow-primary {
  top: 20%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 550px;
  height: 550px;
  background: rgba(99, 102, 241, 0.13);
  animation: pulse-slow 8s ease-in-out infinite alternate;
}

.glow-secondary {
  top: 60%;
  left: 30%;
  transform: translate(-50%, -50%);
  width: 450px;
  height: 450px;
  background: rgba(6, 182, 212, 0.08);
  animation: pulse-slow 10s ease-in-out infinite alternate-reverse;
}

.glow-accent {
  top: 65%;
  left: 70%;
  transform: translate(-50%, -50%);
  width: 380px;
  height: 380px;
  background: rgba(168, 85, 247, 0.07);
}

@keyframes pulse-slow {
  0% { transform: translate(-50%, -50%) scale(1); opacity: 0.8; }
  100% { transform: translate(-50%, -50%) scale(1.15); opacity: 1; }
}

/* 中央绝对居中容器 (顶部预留充足保护空间) */
.welcome-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-start;
  padding: 4.5rem 1.5rem 1.5rem 1.5rem;
  overflow-y: auto;
  z-index: 10;
  width: 100%;
  box-sizing: border-box;
}

.welcome-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  max-width: 820px;
  width: 100%;
  margin: auto 0;
  padding-top: 1rem;
  text-align: center;
}

/* 顶部 Logo 与 Badge 标识 */
.hero-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  margin-top: 1rem;
  margin-bottom: 0.75rem;
}

.logo-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
}

.logo-glow-ring {
  position: absolute;
  width: 72px;
  height: 72px;
  border-radius: 22px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.6), rgba(6, 182, 212, 0.6));
  filter: blur(12px);
  opacity: 0.65;
  transition: opacity 0.3s ease;
}

.hero-logo-box {
  position: relative;
  width: 64px;
  height: 64px;
  border-radius: 18px;
  background: linear-gradient(135deg, #6366f1 0%, #4f46e5 50%, #06b6d4 100%);
  border: 1px solid rgba(255, 255, 255, 0.25);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  box-shadow: 0 12px 32px rgba(99, 102, 241, 0.4);
  transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  z-index: 1;
}

.logo-wrapper:hover .hero-logo-box {
  transform: scale(1.06) rotate(4deg);
}

.logo-wrapper:hover .logo-glow-ring {
  opacity: 0.9;
}

.hero-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 14px;
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.28);
  border-radius: 9999px;
  color: #fbbf24;
  font-size: 0.71875rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  backdrop-filter: blur(8px);
}

/* 标题与描述 */
.welcome-title {
  font-size: 2.125rem;
  font-weight: 900;
  letter-spacing: -0.025em;
  margin-bottom: 0.4rem;
  background: linear-gradient(135deg, #ffffff 20%, #e2e8f0 60%, #818cf8 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  drop-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
}

.github-hero-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 5px 14px;
  margin-bottom: 0.65rem;
  background: rgba(99, 102, 241, 0.12);
  border: 1px solid rgba(99, 102, 241, 0.32);
  border-radius: 9999px;
  color: #c7d2fe;
  font-size: 0.8125rem;
  font-weight: 600;
  text-decoration: none;
  backdrop-filter: blur(12px);
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  box-shadow: 0 4px 16px rgba(99, 102, 241, 0.12);
}

.github-hero-badge:hover {
  background: rgba(99, 102, 241, 0.22);
  border-color: rgba(129, 140, 248, 0.65);
  color: #ffffff;
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(99, 102, 241, 0.3);
}

.badge-icon-gh {
  color: #a5b4fc;
}

.badge-repo {
  text-decoration: underline;
  text-decoration-color: rgba(129, 140, 248, 0.5);
  text-underline-offset: 3px;
  font-weight: 700;
}

.badge-ext {
  opacity: 0.75;
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.github-hero-badge:hover .badge-ext {
  opacity: 1;
  transform: translate(1px, -1px);
}

.welcome-desc {
  font-size: 0.8125rem;
  color: #94a3b8;
  max-width: 620px;
  line-height: 1.6;
  margin-bottom: 1.25rem;
}

.welcome-desc strong {
  color: #cbd5e1;
  font-weight: 600;
}

/* 核心技术卡片网格 (2 列响应式自适应) */
.prompts-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1rem;
  width: 100%;
  margin-bottom: 1.25rem;
}

.prompt-card {
  position: relative;
  padding: 1.125rem 1.25rem;
  background: rgba(13, 18, 36, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 18px;
  cursor: pointer;
  text-align: left;
  backdrop-filter: blur(20px);
  transition: all 0.28s cubic-bezier(0.16, 1, 0.3, 1);
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  overflow: hidden;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.2);
}

.card-glow-line {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  opacity: 0.3;
  transition: opacity 0.3s ease, height 0.3s ease;
}

.prompt-card:hover .card-glow-line {
  opacity: 1;
  height: 2.5px;
}

.prompt-card:hover {
  background: rgba(18, 25, 48, 0.88);
  border-color: rgba(99, 102, 241, 0.45);
  transform: translateY(-4px) scale(1.008);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.45);
}

.card-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.card-icon-box {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: transform 0.25s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.prompt-card:hover .card-icon-box {
  transform: scale(1.12);
}

/* 主题颜色系 */
.theme-indigo .card-glow-line,
.theme-indigo.card-icon-box {
  background: rgba(99, 102, 241, 0.2);
  color: #818cf8;
}

.theme-amber .card-glow-line,
.theme-amber.card-icon-box {
  background: rgba(245, 158, 11, 0.2);
  color: #fbbf24;
}

.theme-cyan .card-glow-line,
.theme-cyan.card-icon-box {
  background: rgba(6, 182, 212, 0.2);
  color: #22d3ee;
}

.theme-purple .card-glow-line,
.theme-purple.card-icon-box {
  background: rgba(168, 85, 247, 0.2);
  color: #c084fc;
}

.tech-pill {
  font-size: 0.6875rem;
  font-weight: 700;
  padding: 2.5px 9px;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.09);
  color: #94a3b8;
  letter-spacing: 0.02em;
}

.card-title {
  font-size: 0.875rem;
  font-weight: 700;
  color: #f1f5f9;
  line-height: 1.35;
  transition: color 0.2s ease;
}

.prompt-card:hover .card-title {
  color: #ffffff;
}

.card-content {
  font-size: 0.75rem;
  color: #94a3b8;
  line-height: 1.55;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-footer-action {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.71875rem;
  font-weight: 600;
  color: #6366f1;
  margin-top: 0.15rem;
  opacity: 0.8;
  transition: opacity 0.2s ease, color 0.2s ease;
}

.prompt-card:hover .card-footer-action {
  opacity: 1;
  color: #818cf8;
}

.action-arrow {
  transition: transform 0.2s ease;
}

.prompt-card:hover .action-arrow {
  transform: translateX(5px);
}

/* 底部技术栈横幅 */
.tech-stack-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 0.65rem 1rem;
  padding: 8px 18px;
  background: rgba(15, 20, 36, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 9999px;
  backdrop-filter: blur(12px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}

.tech-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.71875rem;
  font-weight: 600;
  color: #cbd5e1;
  transition: color 0.2s ease;
}

.tech-tag:hover {
  color: #ffffff;
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  box-shadow: 0 0 6px currentColor;
}

.dot.green { background: #10b981; color: #10b981; }
.dot.blue { background: #3b82f6; color: #3b82f6; }
.dot.purple { background: #a855f7; color: #a855f7; }
.dot.amber { background: #f59e0b; color: #f59e0b; }
.dot.cyan { background: #06b6d4; color: #06b6d4; }

/* 📱 响应式自适应 (Responsive Breakpoints) */
@media (max-width: 768px) {
  .welcome-container {
    padding: 1.25rem 1rem;
  }

  .welcome-title {
    font-size: 1.75rem;
  }

  .prompts-grid {
    grid-template-columns: 1fr;
    gap: 0.85rem;
  }

  .tech-stack-bar {
    border-radius: 16px;
    padding: 10px 14px;
    gap: 0.5rem 0.75rem;
  }
}

@media (max-width: 480px) {
  .welcome-title {
    font-size: 1.5rem;
  }

  .welcome-desc {
    font-size: 0.75rem;
  }

  .prompt-card {
    padding: 1rem;
  }
}
</style>



