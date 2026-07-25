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
  // 获取历史会话列表
  listSessions(params?: ListSessionsRequest): Promise<ListSessionsResponse> {
    return http.get('/nocli/v1/sessions', { params });
  },

  // 删除指定会话
  deleteSession(data: DeleteSessionRequest): Promise<DeleteSessionResponse> {
    return http.post('/nocli/v1/session/delete', data);
  },

  // 获取会话历史记录与中断状态
  getSessionHistory(params: GetSessionHistoryRequest): Promise<GetSessionHistoryResponse> {
    return http.get('/nocli/v1/session/history', { params });
  },
};
