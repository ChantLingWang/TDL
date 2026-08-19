// ============================================================================
// 聊天域类型 —— 与后端 WS 推送 / REST 返回对齐的唯一来源
// 字段说明对应 docs/message-chain.md；新增字段时同步更新这里与文档。
// ============================================================================

export type ConversationType = 'private' | 'group' | 'ai' | 'ai-research';

/** 群组（含 AI 会话，AI 会话即 group_type 为 ai / ai-research 的群） */
export interface Group {
  group_id: string;
  group_name: string;
  group_type: string; // 'normal' | 'ai' | 'ai-research'
  create_by_user_id?: string;
  create_time?: string;
}

/**
 * WS 推送的单条消息。
 * 现状字段：type/content/time/message_id/reply_to_msg_id/metadata
 * 阶段三预留：kind/seq —— AiReplyDelta 增量分块字段
 */
export interface WsMessage {
  type: string;
  group_id?: string;
  conversation_id?: string;
  sender?: string;
  content?: string;
  time?: number;
  message_id?: string;
  reply_to_msg_id?: string;
  metadata?: Record<string, string>;
  /** delta 分块类型（阶段三启用）：thinking=思考链 / progress=进度 / content=正文 / done=结束 */
  kind?: 'thinking' | 'progress' | 'content' | 'done';
  /** delta 递增序号，前端按 (message_id, seq) 累积排序 */
  seq?: number;
  /** 历史消息里带出的思考全文（后端 metadata.reasoning 的转出） */
  reasoning?: string;
}

/** 发送到 WS 的载荷（与 message-chain.md §2 对齐） */
export interface SendPayload {
  type: 'chat';
  content: {
    sender_id: string;
    text: string;
    message_id: string;
    message_type: string;
    conversation_type: ConversationType;
    group_id: string;
  };
}

/** 判断群是否为 AI 会话 */
export const isAIGroup = (g?: Group | null): boolean =>
  !!g && (g.group_type === 'ai' || g.group_type === 'ai-research');
