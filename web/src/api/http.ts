import axios from 'axios';
import type { CommonResponse } from '../types/api';

// 创建 Axios 实例
export const http = axios.create({
  baseURL: '', // 使用相对路径支持 Vite proxy 开发代理
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// 请求拦截器：自动注入 Bearer Token
http.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('auth_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// 响应拦截器：自动拆包 common.proto 响应格式 { code, message, data }
http.interceptors.response.use(
  (response) => {
    const res: CommonResponse = response.data;
    // 如果返回了标准 response 且 code 为 0/200，解出 data
    if (res && typeof res.code !== 'undefined') {
      if (res.code === 0 || res.code === 200) {
        return res.data;
      } else {
        const errorMsg = res.message || res.reason || '请求服务发生错误';
        return Promise.reject(new Error(errorMsg));
      }
    }
    return response.data;
  },
  (error) => {
    if (error.response && error.response.status === 401) {
      localStorage.removeItem('auth_token');
      // 如果未授权跳转登录
      if (!window.location.pathname.startsWith('/auth')) {
        window.location.href = '/auth';
      }
    }
    const message = error.response?.data?.message || error.message || '网络连接异常';
    return Promise.reject(new Error(message));
  }
);
