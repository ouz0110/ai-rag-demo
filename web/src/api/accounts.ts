import { http } from './http';
import type {
  RegisterRequest,
  RegisterResponse,
  LoginRequest,
  LoginResponse,
  ChangePasswordRequest,
  Account,
} from '../types/api';

export const accountsApi = {
  // 注册账号
  register(data: RegisterRequest): Promise<RegisterResponse> {
    return http.post('/base/v1/accounts/register', data);
  },

  // 登录
  login(data: LoginRequest): Promise<LoginResponse> {
    return http.post('/base/v1/accounts/login', data);
  },

  // 登出
  logout(): Promise<void> {
    return http.post('/base/v1/accounts/logout');
  },

  // 修改密码
  changePassword(data: ChangePasswordRequest): Promise<void> {
    return http.post('/base/v1/accounts/changePassword', data);
  },

  // 获取用户信息
  getUserInfo(): Promise<Account> {
    return http.get('/base/v1/accounts/userInfo');
  },
};
