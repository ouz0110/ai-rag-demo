<template>
  <aside class="sidebar">
    <!-- 顶部 Brand & 新建对话 -->
    <div class="sidebar-header">
      <div class="brand-box">
        <div class="logo-icon">
          <Bot :size="20" />
        </div>
        <div class="brand-info">
          <span class="brand-title">AI-RAG-DEMO</span>
          <span class="brand-sub">Agent Workbench</span>
        </div>
      </div>

      <div class="flex flex-col gap-2">
        <button @click="createNewChat" class="new-chat-btn">
          <Plus :size="16" />
          <span>发起新对话</span>
        </button>

        <router-link to="/kb" class="kb-workbench-btn">
          <Database :size="16" />
          <span>知识库管理中心</span>
        </router-link>
      </div>
    </div>

    <!-- 中间：会话历史列表 -->
    <div class="sidebar-body">
      <div class="section-title">
        <span>历史会话记录</span>
        <span class="count-badge">{{ chatStore.sessions.length }}</span>
      </div>

      <div v-if="chatStore.isSessionsLoading" class="state-box">
        <Loader2 :size="16" class="animate-spin" />
        <span>加载历史记录...</span>
      </div>

      <div v-else-if="chatStore.sessions.length === 0" class="state-box empty">
        <MessageSquare :size="24" class="empty-icon" />
        <span>暂无对话记录</span>
        <p>点击上方按钮开启全新对话</p>
      </div>

      <div v-else class="session-list">
        <div
          v-for="s in chatStore.sessions"
          :key="s.session_id"
          @click="selectSession(s.session_id)"
          :class="['session-item', chatStore.currentSessionId === s.session_id && 'active']"
        >
          <div class="item-main">
            <MessageSquare :size="15" class="item-icon" />
            <!-- 中断审批指示灯 -->
            <span
              v-if="s.status === SessionStatus.SS_INTERRUPTED"
              class="interrupt-dot"
              title="等待人工授权审批"
            ></span>
            <span class="item-name">{{ s.name || '新会话 ' + s.session_id.slice(0, 6) }}</span>
          </div>

          <!-- 删除按钮 -->
          <button
            @click.stop="handleDelete(s.session_id)"
            class="delete-btn"
            title="删除会话"
          >
            <Trash2 :size="14" />
          </button>
        </div>
      </div>
    </div>

    <!-- 底部：用户信息栏 -->
    <div class="sidebar-footer">
      <div class="user-card">
        <div class="avatar-circle">
          {{ userAvatarText }}
        </div>
        <div class="user-info">
          <p class="user-name">{{ userNameText }}</p>
          <p class="user-id">ID: {{ userIdText }}</p>
        </div>
      </div>

      <button @click="handleLogout" class="logout-btn" title="退出登录">
        <LogOut :size="16" />
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { Bot, Plus, MessageSquare, Trash2, LogOut, Loader2, Database } from 'lucide-vue-next';
import { useChatStore } from '../../stores/chat';
import { useUserStore } from '../../stores/user';
import { SessionStatus } from '../../types/api';

const router = useRouter();
const chatStore = useChatStore();
const userStore = useUserStore();

const userAvatarText = computed(() => {
  const name = userStore.userInfo?.nickname || userStore.userInfo?.account || 'U';
  return name[0].toUpperCase();
});

const userNameText = computed(() => {
  return userStore.userInfo?.nickname || userStore.userInfo?.account || '用户';
});

const userIdText = computed(() => {
  const openid = userStore.userInfo?.openid || userStore.userInfo?.account || '';
  if (openid.length > 10) {
    return openid.slice(0, 8) + '...';
  }
  return openid || 'admin';
});

onMounted(() => {
  chatStore.fetchSessions();
  userStore.fetchUserInfo();
});

function createNewChat() {
  chatStore.resetCurrentChat();
  router.push('/chat');
}

function selectSession(sessionId: string) {
  if (!sessionId || sessionId === 'undefined') {
    router.push('/chat');
    return;
  }
  chatStore.selectSession(sessionId);
  router.push(`/chat/${sessionId}`);
}

async function handleDelete(sessionId: string) {
  if (confirm('确认要删除此会话记录吗？')) {
    await chatStore.deleteSession(sessionId);
    if (chatStore.currentSessionId === '') {
      router.push('/chat');
    }
  }
}

async function handleLogout() {
  await userStore.logout();
  router.push('/auth');
}
</script>

<style scoped>
.sidebar {
  width: 270px;
  min-width: 270px;
  height: 100%;
  background-color: #090c17;
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  user-select: none;
  z-index: 20;
}

.sidebar-header {
  padding: 1.25rem 1rem 1rem 1rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.brand-box {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logo-icon {
  width: 36px;
  height: 36px;
  border-radius: 12px;
  background: linear-gradient(135deg, #6366f1, #06b6d4);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
}

.brand-info {
  display: flex;
  flex-direction: column;
}

.brand-title {
  font-size: 1rem;
  font-weight: 800;
  color: #f8fafc;
  letter-spacing: 0.03em;
}

.brand-sub {
  font-size: 0.65rem;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.new-chat-btn {
  width: 100%;
  height: 40px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.2), rgba(79, 70, 229, 0.2));
  border: 1px solid rgba(99, 102, 241, 0.4);
  color: #a5b4fc;
  border-radius: 12px;
  font-size: 0.8125rem;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.new-chat-btn:hover {
  background: linear-gradient(135deg, #6366f1, #4f46e5);
  color: #ffffff;
  border-color: transparent;
  box-shadow: 0 4px 14px rgba(99, 102, 241, 0.4);
}

.kb-workbench-btn {
  width: 100%;
  height: 36px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #94a3b8;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  text-decoration: none;
}

.kb-workbench-btn:hover {
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.4);
  color: #22d3ee;
}

/* 列表区 */
.sidebar-body {
  flex: 1;
  overflow-y: auto;
  padding: 1rem 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.section-title {
  padding: 0 0.5rem 0.5rem 0.5rem;
  font-size: 0.7rem;
  font-weight: 700;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.count-badge {
  padding: 2px 7px;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.06);
  color: #94a3b8;
  font-size: 0.65rem;
  font-weight: 600;
}

.state-box {
  padding: 2rem 1rem;
  text-align: center;
  color: #64748b;
  font-size: 0.75rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.state-box.empty .empty-icon {
  color: #334155;
  margin-bottom: 4px;
}

.state-box.empty p {
  font-size: 0.7rem;
  color: #475569;
}

.session-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.session-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
  color: #94a3b8;
  border: 1px solid transparent;
}

.session-item:hover {
  background: rgba(255, 255, 255, 0.04);
  color: #f1f5f9;
}

.session-item.active {
  background: rgba(99, 102, 241, 0.15);
  border-color: rgba(99, 102, 241, 0.3);
  color: #818cf8;
  font-weight: 600;
}

.item-main {
  display: flex;
  align-items: center;
  gap: 10px;
  overflow: hidden;
}

.item-icon {
  flex-shrink: 0;
  color: #64748b;
}

.session-item.active .item-icon {
  color: #818cf8;
}

.interrupt-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background-color: #fbbf24;
  box-shadow: 0 0 6px #fbbf24;
  flex-shrink: 0;
}

.item-name {
  font-size: 0.8125rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.delete-btn {
  opacity: 0;
  background: transparent;
  border: none;
  color: #64748b;
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.session-item:hover .delete-btn {
  opacity: 1;
}

.delete-btn:hover {
  color: #f87171;
  background: rgba(239, 68, 68, 0.15);
}

/* 底部 User 栏 */
.sidebar-footer {
  padding: 0.875rem 1rem;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  background-color: #070912;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.user-card {
  display: flex;
  align-items: center;
  gap: 10px;
  overflow: hidden;
}

.avatar-circle {
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
  flex-shrink: 0;
  box-shadow: 0 2px 8px rgba(99, 102, 241, 0.3);
}

.user-info {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.user-name {
  font-size: 0.8125rem;
  font-weight: 700;
  color: #f1f5f9;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-id {
  font-size: 0.65rem;
  color: #64748b;
  font-family: monospace;
}

.logout-btn {
  background: transparent;
  border: none;
  color: #64748b;
  padding: 6px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.logout-btn:hover {
  color: #f87171;
  background: rgba(239, 68, 68, 0.15);
}
</style>

