// ============================================================================
// 聊天域类型 —— AI-only
// ============================================================================

/** AI 会话（前端仍沿用 group_id 作为内部会话 ID） */
export interface Group {
  group_id: string;
  group_name: string;
}

/** WS 推送的单条消息 */
export interface WsMessage {
  type: string;
  conversation_id?: string;
  group_id?: string;
  sender?: string;
  content?: string;
  time?: number;
  message_id?: string;
  reply_to_msg_id?: string;
  metadata?: Record<string, string>;
  /** delta 分块类型 */
  kind?: 'thinking' | 'progress' | 'content' | 'done';
  /** delta 递增序号 */
  seq?: number;
  /** 历史消息里带出的思考全文 */
  reasoning?: string;
}

/** 发送到 WS 的载荷 */
export interface SendPayload {
  type: 'chat';
  content: {
    text: string;
    message_id: string;
    message_type: string;
    conversation_id: string;
  };
}
