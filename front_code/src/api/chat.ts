// ============================================================================
// chat gateway REST 接口封装（axios）
// ============================================================================
import axios from 'axios';
import type { WsMessage } from '../types/chat';

const chatRequest = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
  headers: { 'Content-Type': 'application/json' },
});

chatRequest.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token');
  if (token) config.headers.Authorization = 'Bearer ' + token;
  return config;
});

chatRequest.interceptors.response.use(
  (response) => response.data,
  (error) => { console.error('chat API error:', error); return Promise.reject(error); },
);

export const chatApi = {
  /** gateway 代理到 auth_service 的用户资料 */
  initUser: (userID: string) =>
    chatRequest.get('/users/' + userID),

  /** 由 gateway 生成 conversation_id，不落库 */
  createConversation: () =>
    chatRequest.post<unknown, { conversation_id: string }>('/conversations'),
};

export type { WsMessage };
