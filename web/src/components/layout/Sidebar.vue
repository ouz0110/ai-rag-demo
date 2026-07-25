<template>
  <aside class="sidebar w-64 h-full bg-[#0e0f12] border-r border-slate-800/80 flex flex-col justify-between shrink-0 select-none">
    <!-- 顶部 logo 与新建对话 -->
    <div class="p-3.5 border-b border-slate-800/60">
      <div class="flex items-center justify-between mb-3 px-1">
        <div class="flex items-center gap-2">
          <div class="w-7 h-7 rounded-lg bg-gradient-to-tr from-indigo-500 to-cyan-400 flex items-center justify-center font-black text-white text-xs shadow-md">
            DS
          </div>
          <span class="font-bold text-sm tracking-wide text-slate-100">DeepSeek Chat</span>
        </div>
      </div>

      <button
        @click="createNewChat"
        class="w-full py-2 px-3 bg-indigo-600/20 hover:bg-indigo-600/30 border border-indigo-500/40 text-indigo-200 rounded-xl text-xs font-semibold flex items-center justify-center gap-2 transition-all"
      >
        <Plus :size="14" />
        <span>开启新对话</span>
      </button>
    </div>

    <!-- 中间：会话历史列表 -->
    <div class="flex-1 overflow-y-auto p-2 space-y-1">
      <div class="px-2 py-1 text-[11px] font-semibold text-slate-500 uppercase tracking-wider">
        历史会话 ({{ chatStore.sessions.length }})
      </div>

      <div v-if="chatStore.isSessionsLoading" class="p-4 text-center text-xs text-slate-500">
        加载历史会话...
      </div>

      <div v-else-if="chatStore.sessions.length === 0" class="p-4 text-center text-xs text-slate-500">
        暂无对话记录
      </div>

      <template v-else>
        <div
          v-for="s in chatStore.sessions"
          :key="s.session_id"
          @click="selectSession(s.session_id)"
          class="group relative flex items-center justify-between px-3 py-2.5 rounded-xl text-xs cursor-pointer transition-all"
          :class="[
            chatStore.currentSessionId === s.session_id
              ? 'bg-slate-800 text-white font-medium border border-slate-700/60'
              : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'
          ]"
        >
          <div class="flex items-center gap-2 overflow-hidden mr-2">
            <MessageSquare :size="13" class="shrink-0 text-slate-500 group-hover:text-indigo-400" />
            <!-- 中断提示红黄指示灯 -->
            <span v-if="s.status === SessionStatus.SS_INTERRUPTED" class="w-2 h-2 rounded-full bg-amber-400 animate-pulse shrink-0" title="该会话需要人工审批"></span>
            <span class="truncate">{{ s.name || '新会话 ' + s.session_id.slice(0, 6) }}</span>
          </div>

          <!-- 删除按钮 -->
          <button
            @click.stop="handleDelete(s.session_id)"
            class="opacity-0 group-hover:opacity-100 p-1 hover:text-red-400 text-slate-500 transition-opacity"
            title="删除会话"
          >
            <Trash2 :size="13" />
          </button>
        </div>
      </template>
    </div>

    <!-- 底部：用户信息栏 -->
    <div class="p-3 border-t border-slate-800/80 bg-[#0a0b0d]">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2 overflow-hidden">
          <div class="w-8 h-8 rounded-full bg-indigo-700 flex items-center justify-center font-bold text-xs text-white shrink-0">
            {{ userStore.userInfo?.nickname?.[0]?.toUpperCase() || userStore.userInfo?.account?.[0]?.toUpperCase() || 'U' }}
          </div>
          <div class="overflow-hidden">
            <p class="text-xs font-semibold text-slate-200 truncate">{{ userStore.userInfo?.nickname || userStore.userInfo?.account || '用户' }}</p>
            <p class="text-[10px] text-slate-500 truncate">ID: {{ userStore.userInfo?.openid || userStore.userInfo?.account }}</p>
          </div>
        </div>

        <button
          @click="handleLogout"
          class="p-1.5 hover:bg-slate-800 text-slate-400 hover:text-red-400 rounded-lg transition-colors"
          title="退出登录"
        >
          <LogOut :size="15" />
        </button>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { Plus, MessageSquare, Trash2, LogOut } from 'lucide-vue-next';
import { useChatStore } from '../../stores/chat';
import { useUserStore } from '../../stores/user';
import { SessionStatus } from '../../types/api';

const router = useRouter();
const chatStore = useChatStore();
const userStore = useUserStore();

onMounted(() => {
  chatStore.fetchSessions();
  userStore.fetchUserInfo();
});

function createNewChat() {
  chatStore.resetCurrentChat();
  router.push('/chat');
}

function selectSession(sessionId: string) {
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
