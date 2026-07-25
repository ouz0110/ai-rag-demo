<template>
  <div class="auth-page w-screen h-screen flex items-center justify-center bg-[#0c0d10] relative overflow-hidden select-none">
    <!-- 装饰背景光效 -->
    <div class="absolute -top-40 -left-40 w-96 h-96 bg-indigo-600/20 rounded-full blur-[120px] pointer-events-none"></div>
    <div class="absolute -bottom-40 -right-40 w-96 h-96 bg-cyan-500/15 rounded-full blur-[120px] pointer-events-none"></div>

    <div class="glass-panel w-full max-w-md p-8 rounded-3xl border border-slate-800 shadow-2xl z-10">
      <!-- 头部 Brand -->
      <div class="text-center mb-8">
        <div class="w-12 h-12 mx-auto mb-3 rounded-2xl bg-gradient-to-tr from-indigo-500 to-cyan-400 flex items-center justify-center font-black text-white text-xl shadow-lg">
          DS
        </div>
        <h2 class="text-2xl font-bold text-slate-100">DeepSeek AI 客户端</h2>
        <p class="text-xs text-slate-400 mt-1">探索大模型深度思考与智能化 Agent 体验</p>
      </div>

      <!-- 模式切换 Tabs -->
      <div class="flex bg-slate-900/80 p-1 rounded-xl mb-6 border border-slate-800 text-xs font-semibold">
        <button
          @click="isRegister = false; errorMsg = ''"
          class="flex-1 py-2 rounded-lg transition-all"
          :class="!isRegister ? 'bg-indigo-600 text-white shadow' : 'text-slate-400 hover:text-slate-200'"
        >
          账号登录
        </button>
        <button
          @click="isRegister = true; errorMsg = ''"
          class="flex-1 py-2 rounded-lg transition-all"
          :class="isRegister ? 'bg-indigo-600 text-white shadow' : 'text-slate-400 hover:text-slate-200'"
        >
          新用户注册
        </button>
      </div>

      <!-- 异常提示框 -->
      <div v-if="errorMsg" class="mb-4 p-3 bg-red-950/40 border border-red-800/50 rounded-xl text-xs text-red-300 flex items-center gap-2">
        <AlertCircle :size="15" />
        <span>{{ errorMsg }}</span>
      </div>

      <!-- 表单区域 -->
      <form @submit.prevent="handleSubmit" class="space-y-4">
        <div>
          <label class="block text-xs font-medium text-slate-300 mb-1">登录账号 / 手机 / 邮箱</label>
          <input
            v-model="form.account"
            type="text"
            required
            placeholder="请输入账号 (至少3位)"
            class="w-full px-3.5 py-2.5 bg-slate-900/90 border border-slate-700/70 rounded-xl text-xs text-slate-100 focus:outline-none focus:border-indigo-500 transition-colors"
          />
        </div>

        <div>
          <label class="block text-xs font-medium text-slate-300 mb-1">密码</label>
          <input
            v-model="form.password"
            type="password"
            required
            placeholder="请输入密码 (至少6位)"
            class="w-full px-3.5 py-2.5 bg-slate-900/90 border border-slate-700/70 rounded-xl text-xs text-slate-100 focus:outline-none focus:border-indigo-500 transition-colors"
          />
        </div>

        <template v-if="isRegister">
          <div>
            <label class="block text-xs font-medium text-slate-300 mb-1">用户昵称</label>
            <input
              v-model="form.nickname"
              type="text"
              placeholder="请输入个性昵称 (可选)"
              class="w-full px-3.5 py-2.5 bg-slate-900/90 border border-slate-700/70 rounded-xl text-xs text-slate-100 focus:outline-none focus:border-indigo-500 transition-colors"
            />
          </div>
        </template>

        <button
          type="submit"
          :disabled="userStore.loading"
          class="w-full py-3 btn-primary text-xs font-semibold rounded-xl mt-2 flex items-center justify-center gap-2 disabled:opacity-50"
        >
          <Loader2 v-if="userStore.loading" :size="16" class="animate-spin" />
          <span>{{ isRegister ? '创建新账号' : '立即登录' }}</span>
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue';
import { useRouter } from 'vue-router';
import { AlertCircle, Loader2 } from 'lucide-vue-next';
import { useUserStore } from '../stores/user';

const router = useRouter();
const userStore = useUserStore();

const isRegister = ref(false);
const errorMsg = ref('');

const form = reactive({
  account: '',
  password: '',
  nickname: '',
});

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
    errorMsg.value = err.message || '操作失败，请重试';
  }
}
</script>
