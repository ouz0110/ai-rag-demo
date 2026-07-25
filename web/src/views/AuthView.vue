<template>
  <div class="auth-page">
    <!-- 全局背景科技光影 -->
    <div class="bg-glow"></div>
    <div class="bg-grid"></div>

    <!-- 左侧：科技展示 banner (桌面端独叠展示) -->
    <div class="hero-section">
      <!-- 光影与卡片装饰 -->
      <div class="orb orb-1"></div>
      <div class="orb orb-2"></div>

      <!-- 顶栏 Brand -->
      <div class="hero-header">
        <div class="brand-logo">
          <Bot :size="24" />
        </div>
        <div class="brand-text">
          <span class="brand-name">AI-RAG-DEMO</span>
          <span class="brand-sub">Next-Gen Agent Platform</span>
        </div>
        <div class="status-badge">
          <span class="status-dot"></span>
          <span>系统在线</span>
        </div>
      </div>

      <!-- 中间主标语与特性列表 -->
      <div class="hero-body">
        <div class="hero-tag">
          <Sparkles :size="14" />
          <span>新一代大模型 Agent 流式工作台</span>
        </div>

        <h1 class="hero-title">
          探索深度推理与<br />
          <span class="gradient-text">流式中断授权审批</span>
        </h1>

        <p class="hero-desc">
          基于 Kratos 高性能 RPC 架构，融合 CoT 深度思考历程、打字机增量响应、RAG 向量知识库以及敏感工具的人工授权。
        </p>

        <!-- 3大核心亮点列表 -->
        <div class="feature-list">
          <div class="feature-card">
            <div class="feature-icon icon-indigo">
              <Brain :size="20" />
            </div>
            <div class="feature-info">
              <h4>CoT 深度推理可视化</h4>
              <p>实时折叠/展开大模型 Chain-of-Thought 底层逻辑思考历程</p>
            </div>
          </div>

          <div class="feature-card">
            <div class="feature-icon icon-cyan">
              <ShieldCheck :size="20" />
            </div>
            <div class="feature-info">
              <h4>敏感 Tool 人工授权审批</h4>
              <p>流式中断挂起，提供同意/拒绝及会话级动态授权范围</p>
            </div>
          </div>

          <div class="feature-card">
            <div class="feature-icon icon-purple">
              <Zap :size="20" />
            </div>
            <div class="feature-info">
              <h4>全流程 SSE 上下文回放</h4>
              <p>高并发增量传输与历史对话切片平滑回放体验</p>
            </div>
          </div>
        </div>

        <!-- 架构标签 -->
        <div class="tech-tags">
          <span class="tag">Go / Kratos</span>
          <span class="tag">Vue 3 / Vite</span>
          <span class="tag">RAG Vector DB</span>
          <span class="tag">SSE Streaming</span>
        </div>
      </div>

      <!-- 底栏版权 -->
      <div class="hero-footer">
        <span>AI RAG Demo Platform &copy; 2026</span>
        <span>v1.0.0 Stable</span>
      </div>
    </div>

    <!-- 右侧：登录/注册表单区 -->
    <div class="form-section">
      <div class="form-container">
        <!-- 移动端 Logo Header -->
        <div class="mobile-header">
          <div class="mobile-logo">
            <Bot :size="20" />
          </div>
          <span class="mobile-title">AI-RAG-DEMO</span>
        </div>

        <!-- 卡片头部标题 -->
        <div class="form-header">
          <h2 class="form-title">
            {{ isRegister ? '创建新账号' : '欢迎回来' }}
          </h2>
          <p class="form-subtitle">
            {{ isRegister ? '请填写您的凭据以开启智能 AI Agent 推导体验' : '请输入您的凭据登录 AI Agent 控制台' }}
          </p>
        </div>

        <!-- Tabs 模式切换 -->
        <div class="tab-switch">
          <button
            type="button"
            @click="isRegister = false; errorMsg = ''"
            :class="['tab-btn', !isRegister && 'active']"
          >
            <KeyRound :size="16" />
            <span>账号登录</span>
          </button>
          <button
            type="button"
            @click="isRegister = true; errorMsg = ''"
            :class="['tab-btn', isRegister && 'active']"
          >
            <User :size="16" />
            <span>注册新账号</span>
          </button>
        </div>

        <!-- 快捷测试账号按钮 (仅登录时显示) -->
        <div v-if="!isRegister" class="quick-fill-container">
          <button type="button" @click="fillQuickAccount" class="quick-fill-btn">
            <Sparkles :size="13" />
            <span>一键填入测试账号 (admin / 123456)</span>
          </button>
        </div>

        <!-- 错误提示框 -->
        <div v-if="errorMsg" class="error-alert">
          <AlertCircle :size="18" class="error-icon" />
          <span>{{ errorMsg }}</span>
        </div>

        <!-- 表单区 -->
        <form @submit.prevent="handleSubmit" class="auth-form">
          <!-- 账号 -->
          <div class="input-group">
            <label class="input-label">
              登录账号 / 用户名 <span class="required-star">*</span>
            </label>
            <div class="input-wrapper">
              <div class="field-icon">
                <User :size="18" />
              </div>
              <input
                v-model="form.account"
                type="text"
                required
                autocomplete="username"
                placeholder="请输入账号 (至少3位)"
                class="form-input"
              />
            </div>
          </div>

          <!-- 密码 -->
          <div class="input-group">
            <label class="input-label">
              登录密码 <span class="required-star">*</span>
            </label>
            <div class="input-wrapper">
              <div class="field-icon">
                <Lock :size="18" />
              </div>
              <input
                v-model="form.password"
                :type="showPassword ? 'text' : 'password'"
                required
                autocomplete="current-password"
                placeholder="请输入密码 (至少6位)"
                class="form-input"
              />
              <button
                type="button"
                @click="showPassword = !showPassword"
                class="eye-toggle-btn"
                title="切换密码显示"
              >
                <Eye v-if="!showPassword" :size="18" />
                <EyeOff v-else :size="18" />
              </button>
            </div>
          </div>

          <!-- 注册专有字段：昵称 -->
          <div v-if="isRegister" class="input-group">
            <label class="input-label">
              用户昵称 <span class="optional-text">(可选)</span>
            </label>
            <div class="input-wrapper">
              <div class="field-icon">
                <Smile :size="18" />
              </div>
              <input
                v-model="form.nickname"
                type="text"
                placeholder="请输入您的个性昵称 (默认同账号)"
                class="form-input"
              />
            </div>
          </div>

          <!-- 提交按钮 -->
          <button
            type="submit"
            :disabled="userStore.loading"
            class="submit-btn"
          >
            <Loader2 v-if="userStore.loading" :size="18" class="spin-icon" />
            <template v-else>
              <span>{{ isRegister ? '确认注册并自动登录' : '立即进入 Agent 控制台' }}</span>
              <ArrowRight :size="18" />
            </template>
          </button>
        </form>

        <!-- 底部说明 -->
        <div class="form-footer">
          <p>登录即视为同意 AI-RAG-DEMO 服务协议与隐私保护条款</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue';
import { useRouter } from 'vue-router';
import {
  Bot,
  Sparkles,
  Brain,
  ShieldCheck,
  Zap,
  User,
  Lock,
  Smile,
  AlertCircle,
  Loader2,
  Eye,
  EyeOff,
  KeyRound,
  ArrowRight
} from 'lucide-vue-next';
import { useUserStore } from '../stores/user';

const router = useRouter();
const userStore = useUserStore();

const isRegister = ref(false);
const showPassword = ref(false);
const errorMsg = ref('');

const form = reactive({
  account: '',
  password: '',
  nickname: '',
});

function fillQuickAccount() {
  form.account = 'admin';
  form.password = '123456';
  errorMsg.value = '';
}

async function handleSubmit() {
  errorMsg.value = '';
  try {
    if (isRegister.value) {
      await userStore.register({
        account: form.account,
        password: form.password,
        nickname: form.nickname || form.account,
      });
    } else {
      await userStore.login({
        account: form.account,
        password: form.password,
      });
    }
    router.push('/chat');
  } catch (err: any) {
    errorMsg.value = err.message || '操作失败，请重新检查账号格式与密码';
  }
}
</script>

<style scoped>
.auth-page {
  position: relative;
  width: 100vw;
  height: 100vh;
  min-height: 100vh;
  display: flex;
  background-color: #060810;
  color: #f8fafc;
  overflow: hidden;
  user-select: none;
  font-family: 'Inter', system-ui, -apple-system, sans-serif;
}

.bg-glow {
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse 80% 70% at 50% -20%, rgba(99, 102, 241, 0.18), transparent);
  pointer-events: none;
}

.bg-grid {
  position: absolute;
  inset: 0;
  background-image: 
    linear-gradient(to right, rgba(255, 255, 255, 0.03) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(255, 255, 255, 0.03) 1px, transparent 1px);
  background-size: 3.5rem 3.5rem;
  mask-image: radial-gradient(ellipse 60% 50% at 50% 0%, #000 70%, transparent 100%);
  pointer-events: none;
}

/* 左侧 Hero 样式 */
.hero-section {
  position: relative;
  width: 58%;
  height: 100%;
  padding: 3rem;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  background: linear-gradient(135deg, #090c18 0%, #060812 50%, #04050a 100%);
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  z-index: 10;
  overflow: hidden;
}

@media (max-width: 1024px) {
  .hero-section {
    display: none;
  }
}

.orb {
  position: absolute;
  border-radius: 50%;
  pointer-events: none;
}

.orb-1 {
  top: -6rem;
  left: -6rem;
  width: 500px;
  height: 500px;
  background: rgba(99, 102, 241, 0.15);
  filter: blur(140px);
}

.orb-2 {
  bottom: -4rem;
  right: -4rem;
  width: 450px;
  height: 450px;
  background: rgba(6, 182, 212, 0.12);
  filter: blur(140px);
}

.hero-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  z-index: 10;
}

.brand-logo {
  width: 44px;
  height: 44px;
  border-radius: 14px;
  background: linear-gradient(135deg, #6366f1, #06b6d4);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  box-shadow: 0 4px 16px rgba(99, 102, 241, 0.4);
}

.brand-text {
  margin-left: 12px;
  display: flex;
  flex-direction: column;
}

.brand-name {
  font-size: 1.25rem;
  font-weight: 900;
  letter-spacing: 0.05em;
  color: #ffffff;
}

.brand-sub {
  font-size: 0.65rem;
  font-weight: 700;
  letter-spacing: 0.1em;
  color: #64748b;
  text-transform: uppercase;
}

.status-badge {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border-radius: 9999px;
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.25);
  color: #34d399;
  font-size: 0.75rem;
  font-weight: 600;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: #34d399;
  box-shadow: 0 0 8px #34d399;
}

.hero-body {
  margin: auto 0;
  z-index: 10;
  max-width: 560px;
}

.hero-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border-radius: 9999px;
  background: rgba(99, 102, 241, 0.12);
  border: 1px solid rgba(99, 102, 241, 0.3);
  color: #a5b4fc;
  font-size: 0.75rem;
  font-weight: 600;
  margin-bottom: 1.5rem;
}

.hero-title {
  font-size: 2.75rem;
  font-weight: 900;
  line-height: 1.2;
  color: #f8fafc;
  letter-spacing: -0.02em;
  margin-bottom: 1.25rem;
}

.gradient-text {
  background: linear-gradient(135deg, #818cf8 0%, #c084fc 50%, #22d3ee 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.hero-desc {
  font-size: 0.875rem;
  color: #94a3b8;
  line-height: 1.6;
  margin-bottom: 2rem;
}

.feature-list {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.feature-card {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 1rem 1.25rem;
  background: rgba(15, 23, 42, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  backdrop-filter: blur(12px);
  transition: all 0.25s ease;
}

.feature-card:hover {
  border-color: rgba(99, 102, 241, 0.4);
  background: rgba(15, 23, 42, 0.8);
  transform: translateY(-2px);
}

.feature-icon {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.icon-indigo {
  background: rgba(99, 102, 241, 0.15);
  color: #818cf8;
  border: 1px solid rgba(99, 102, 241, 0.25);
}

.icon-cyan {
  background: rgba(6, 182, 212, 0.15);
  color: #22d3ee;
  border: 1px solid rgba(6, 182, 212, 0.25);
}

.icon-purple {
  background: rgba(168, 85, 247, 0.15);
  color: #c084fc;
  border: 1px solid rgba(168, 85, 247, 0.25);
}

.feature-info h4 {
  font-size: 0.8125rem;
  font-weight: 700;
  color: #f1f5f9;
  margin-bottom: 2px;
}

.feature-info p {
  font-size: 0.75rem;
  color: #94a3b8;
  line-height: 1.4;
}

.tech-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 1.75rem;
}

.tag {
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(30, 41, 59, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #94a3b8;
  font-size: 0.7rem;
  font-family: monospace;
}

.hero-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.75rem;
  color: #64748b;
  font-family: monospace;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  padding-top: 1rem;
  z-index: 10;
}

/* 右侧 Form 区域样式 */
.form-section {
  position: relative;
  width: 42%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2.5rem 2rem;
  background-color: #080a13;
  z-index: 10;
  overflow-y: auto;
}

@media (max-width: 1024px) {
  .form-section {
    width: 100%;
  }
}

.form-container {
  width: 100%;
  max-width: 420px;
  margin: auto 0;
  display: flex;
  flex-direction: column;
}

.mobile-header {
  display: none;
  align-items: center;
  gap: 10px;
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

@media (max-width: 1024px) {
  .mobile-header {
    display: flex;
  }
}

.mobile-logo {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: #6366f1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
}

.mobile-title {
  font-size: 1.125rem;
  font-weight: 900;
  color: #ffffff;
}

.form-header {
  margin-bottom: 1.5rem;
}

.form-title {
  font-size: 1.75rem;
  font-weight: 800;
  color: #f8fafc;
  letter-spacing: -0.02em;
  margin-bottom: 0.5rem;
}

.form-subtitle {
  font-size: 0.8125rem;
  color: #94a3b8;
  line-height: 1.5;
}

/* Tabs */
.tab-switch {
  display: flex;
  background: #0d101d;
  padding: 5px;
  border-radius: 14px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  margin-bottom: 1.25rem;
}

.tab-btn {
  flex: 1;
  height: 42px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: #94a3b8;
  font-size: 0.875rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.tab-btn.active {
  background: linear-gradient(135deg, #6366f1, #4f46e5);
  color: #ffffff;
  box-shadow: 0 4px 14px rgba(99, 102, 241, 0.35);
}

.tab-btn:hover:not(.active) {
  color: #f1f5f9;
}

.quick-fill-container {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 1rem;
}

.quick-fill-btn {
  background: transparent;
  border: none;
  color: #818cf8;
  font-size: 0.75rem;
  font-weight: 600;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  transition: color 0.2s;
}

.quick-fill-btn:hover {
  color: #a5b4fc;
  text-decoration: underline;
}

.error-alert {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  background: rgba(127, 29, 29, 0.35);
  border: 1px solid rgba(239, 68, 68, 0.4);
  border-radius: 12px;
  color: #fca5a5;
  font-size: 0.8125rem;
  margin-bottom: 1.25rem;
}

.error-icon {
  color: #f87171;
  flex-shrink: 0;
}

/* 输入框核心机制 (彻底解决错位与覆盖问题) */
.auth-form {
  display: flex;
  flex-direction: column;
  gap: 1.125rem;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.input-label {
  font-size: 0.75rem;
  font-weight: 700;
  color: #cbd5e1;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.required-star {
  color: #818cf8;
}

.optional-text {
  color: #64748b;
  font-weight: 400;
  text-transform: none;
}

.input-wrapper {
  position: relative;
  width: 100%;
  display: flex;
  align-items: center;
}

.field-icon {
  position: absolute;
  left: 14px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 10;
  pointer-events: none;
  color: #64748b;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.2s ease;
}

.input-wrapper:focus-within .field-icon {
  color: #818cf8;
}

.form-input {
  width: 100% !important;
  height: 48px !important;
  padding-left: 46px !important; /* 关键：给左侧图标预留 46px */
  padding-right: 46px !important; /* 关键：给右侧切换密码按钮预留 46px */
  padding-top: 0 !important;
  padding-bottom: 0 !important;
  background-color: #0b0e1b !important;
  border: 1px solid #1e293b !important;
  border-radius: 12px !important;
  color: #f8fafc !important;
  font-size: 0.875rem !important;
  line-height: 48px !important;
  outline: none !important;
  box-sizing: border-box !important;
  transition: all 0.2s ease !important;
}

.form-input::placeholder {
  color: #475569 !important;
}

.form-input:focus {
  border-color: #6366f1 !important;
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.25) !important;
  background-color: #0e1224 !important;
}

.eye-toggle-btn {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 10;
  background: transparent;
  border: none;
  color: #64748b;
  padding: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  transition: color 0.2s ease;
}

.eye-toggle-btn:hover {
  color: #cbd5e1;
}

.submit-btn {
  width: 100%;
  height: 48px;
  margin-top: 0.5rem;
  background: linear-gradient(135deg, #6366f1 0%, #4f46e5 100%);
  border: none;
  border-radius: 12px;
  color: #ffffff;
  font-size: 0.9375rem;
  font-weight: 700;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  box-shadow: 0 4px 16px rgba(99, 102, 241, 0.35);
  transition: all 0.2s ease;
}

.submit-btn:hover:not(:disabled) {
  background: linear-gradient(135deg, #818cf8 0%, #6366f1 100%);
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(99, 102, 241, 0.45);
}

.submit-btn:active:not(:disabled) {
  transform: translateY(0);
}

.submit-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.spin-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.form-footer {
  margin-top: 1.5rem;
  padding-top: 1rem;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  text-align: center;
  font-size: 0.75rem;
  color: #475569;
}
</style>


