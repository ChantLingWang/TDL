// ============================================================================
// chat_service REST 接口封装（axios）
// 类型定义统一在 ../types/chat.ts
// ============================================================================
import axios from 'axios';
import type { Group, WsMessage } from '../types/chat';

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

/** 历史消息接口的原始返回（latest 在前） */
interface HistoryItem extends WsMessage {
  sender_id?: string;
  timestamp?: number | string;
}

export const chatApi = {
  initUser: (userID: string) =>
    chatRequest.get('/users/' + userID),

  getUserGroups: (userID: string) =>
    chatRequest.get<any, { groups: Group[] }>('/users/' + userID + '/groups'),

  createGroup: (groupName: string, creatorID: string, groupType: string = 'normal') =>
    chatRequest.post<any, Group>('/groups', {
      group_name: groupName, creator_id: creatorID, group_type: groupType,
    }),

  getGroupMembers: (groupID: string) =>
    chatRequest.get<unknown, { members: string[] }>('/groups/' + groupID + '/members'),

  addGroupMember: (groupID: string, userID: string) =>
    chatRequest.post<unknown, { message: string }>('/groups/' + groupID + '/members', { user_id: userID }),

  removeGroupMember: (groupID: string, userID: string) =>
    chatRequest.delete<unknown, { message: string }>('/groups/' + groupID + '/members/' + userID),

  getMessages: () =>
    chatRequest.get<unknown, {
      messages: WsMessage[];
      total_unread_count: number;
      unread_counts: Record<string, number>;
    }>('/messages'),

  markMessagesAsRead: (conversationID: string) =>
    chatRequest.post<unknown, { message: string }>('/messages/read', { conversation_id: conversationID }),

  getHistory: (groupId: string, cursor: number, limit = 50) =>
    chatRequest.get<any, { messages: HistoryItem[] }>(
      '/messages/history', { params: { conversation_id: groupId, cursor, limit } }),
};

export type { Group, WsMessage };
