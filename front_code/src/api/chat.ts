import axios from 'axios';

const chatRequest = axios.create({
  baseURL: "/api/v1",
  timeout: 10000,
  headers: { 'Content-Type': 'application/json' },
});

chatRequest.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token');
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

chatRequest.interceptors.response.use(
  (response) => response.data,
  (error) => { console.error('chat API error:', error); return Promise.reject(error); },
);

export interface Group {
  group_id: string;
  group_name: string;
  group_type: string;
  create_by_user_id?: string;
  create_time?: string;
}

export interface ChatMessage {
  group_id: string;
  sender: string;
  content: string;
  time: number;
  type?: string;
}

export const chatApi = {
  initUser: (userID: string) =>
    chatRequest.get(`/users/${userID}`),

  getUserGroups: (userID: string) =>
    chatRequest.get<any, { groups: Group[] }>(`/users/${userID}/groups`),

  createGroup: (groupName: string, creatorID: string, groupType: string = 'normal') =>
    chatRequest.post<any, Group>('/groups', {
      group_name: groupName, creator_id: creatorID, group_type: groupType,
    }),

  joinGroup: (groupID: string, userID: string) =>
    chatRequest.post('/groups/join', { group_id: groupID, user_id: userID }),

  getMessages: (params?: { group_id?: string; limit?: number }) =>
    chatRequest.get<any, { messages: ChatMessage[]; total_unread_count: number }>(
      '/messages', { params }),

  getHistory: (groupId: string, cursor: number, limit = 50) =>
    chatRequest.get<any, { messages: ChatMessage[] }>(
      '/messages/history', { params: { conversation_id: groupId, cursor, limit } }),
};
