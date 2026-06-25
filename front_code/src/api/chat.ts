import axios from 'axios';

const chatRequest = axios.create({
  baseURL: import.meta.env.VITE_API_URL,
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
  create_time?: string;
}

export interface ChatMessage {
  conversation_id: string;
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

  createGroup: (groupName: string, creatorID: string) =>
    chatRequest.post<any, { group_id: string; group_name: string }>('/groups', {
      group_name: groupName, creator_id: creatorID,
    }),

  joinGroup: (groupID: string, userID: string) =>
    chatRequest.post('/groups/join', { group_id: groupID, user_id: userID }),

  getMessages: (params?: { conversation_id?: string; limit?: number }) =>
    chatRequest.get<any, { messages: ChatMessage[]; total_unread_count: number }>(
      '/messages', { params }),
};
