import { http } from './http';
import type {
  ListSessionsRequest,
  ListSessionsResponse,
  DeleteSessionRequest,
  DeleteSessionResponse,
  GetSessionHistoryRequest,
  GetSessionHistoryResponse,
} from '../types/api';

export const chatApi = {
  // 获取历史会话列表 (POST)
  listSessions(data?: ListSessionsRequest): Promise<ListSessionsResponse> {
    return http.post('/nocli/v1/sessions', data || {});
  },

  // 删除指定会话
  deleteSession(data: DeleteSessionRequest): Promise<DeleteSessionResponse> {
    if (!data || !data.session_id || data.session_id === 'undefined') {
      return Promise.reject(new Error('无效的 session_id'));
    }
    return http.post('/nocli/v1/session/delete', data);
  },

  // 获取会话历史记录与中断状态 (POST)
  getSessionHistory(data: GetSessionHistoryRequest): Promise<GetSessionHistoryResponse> {
    if (!data || !data.session_id || data.session_id === 'undefined') {
      return Promise.reject(new Error('无效的 session_id'));
    }
    return http.post('/nocli/v1/session/history', data);
  },
};

