import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { accountsApi } from '../api/accounts';
import type { Account, LoginRequest, RegisterRequest, ChangePasswordRequest } from '../types/api';

export const useUserStore = defineStore('user', () => {
  const token = ref<string>(localStorage.getItem('auth_token') || '');
  const userInfo = ref<Account | null>(null);
  const loading = ref<boolean>(false);

  const isAuthenticated = computed(() => !!token.value);

  function setToken(newToken: string) {
    token.value = newToken;
    if (newToken) {
      localStorage.setItem('auth_token', newToken);
    } else {
      localStorage.removeItem('auth_token');
    }
  }

  async function login(req: LoginRequest) {
    loading.value = true;
    try {
      const res = await accountsApi.login(req);
      setToken(res.token);
      await fetchUserInfo();
      return res;
    } finally {
      loading.value = false;
    }
  }

  async function register(req: RegisterRequest) {
    loading.value = true;
    try {
      const res = await accountsApi.register(req);
      setToken(res.token);
      await fetchUserInfo();
      return res;
    } finally {
      loading.value = false;
    }
  }

  async function fetchUserInfo() {
    if (!token.value) return;
    try {
      const info = await accountsApi.getUserInfo();
      userInfo.value = info;
    } catch (err) {
      console.error('Failed to fetch user info:', err);
      // token 过期或失效，清空
      setToken('');
    }
  }

  async function changePassword(req: ChangePasswordRequest) {
    await accountsApi.changePassword(req);
  }

  async function logout() {
    try {
      await accountsApi.logout();
    } catch (e) {
      // ignore
    } finally {
      setToken('');
      userInfo.value = null;
    }
  }

  return {
    token,
    userInfo,
    loading,
    isAuthenticated,
    login,
    register,
    fetchUserInfo,
    changePassword,
    logout,
  };
});
