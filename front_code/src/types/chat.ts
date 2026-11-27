// ============================================================================
// 聊天域类型 —— AI-only
// ============================================================================

/** AI 会话（前端仍沿用 group_id 作为内部会话 ID） */
export interface Group {
  group_id: string;
  group_name: string;
}

/** AI 一步的类型 */
export type StepKind = 'THINK' | 'TOOL_CALL' | 'TOOL_RESULT' | 'RESULT';

/** TOOL_CALL 里的单个工具调用 */
export interface ToolCallItem {
  id: string;
  name: string;
  input?: unknown;
  error?: string;
}

/** TOOL_RESULT 里的单个工具返回 */
export interface ToolResultItem {
  id: string;
  name: string;
  payload?: unknown;
  status?: string;
  duration_ms?: number;
  error?: string;
}

/** 一步的 payload */
export interface StepPayload {
  think?: string;
  calls?: ToolCallItem[];
  results?: ToolResultItem[];
  result?: string;
  status?: string;
  reason?: string;
}

/** 一步 */
export interface StepEntry {
  kind: StepKind;
  step: number;
  payload: StepPayload;
  time?: number;
}

/** 一条 AI 回复的 step 流 */
export interface StepStream {
  conversation_id: string;
  reply_to_msg_id?: string;
  message_id: string;
  sender?: string;
  round: number;
  attempt: number;
  steps: StepEntry[];
  done: boolean;
}

/** WS 推送的 ai_step 事件 */
export interface StepMessage {
  type: 'ai_step';
  conversation_id: string;
  sender?: string;
  reply_to_msg_id?: string;
  message_id: string;
  round?: number;
  step: number;
  attempt?: number;
  kind: StepKind;
  payload: StepPayload;
  time?: number;
}

/** WS 推送的单条消息（用户消息 / 最终 AI 消息） */
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
  /** 历史消息里带出的思考全文 */
  reasoning?: string;
  /** 历史消息携带的 step 列表（后续 storage 接入后使用） */
  steps?: StepEntry[];
}

/** 用户消息里的一段内容 */
export type ContentPartType = 'text' | 'image' | 'audio' | 'video';

export interface ContentPart {
  type: ContentPartType;
  text?: string;
  /** base64，gateway 会解码成 bytes */
  data?: string;
  mime_type?: string;
  /** 透传用：宽高、时长、文件名、追踪探针等 */
  metadata?: Record<string, unknown>;
}

/** 发送到 WS 的载荷 */
export interface SendPayload {
  type: 'chat';
  content: {
    parts: ContentPart[];
    message_id: string;
    message_type: string;
    conversation_id: string;
  };
}
