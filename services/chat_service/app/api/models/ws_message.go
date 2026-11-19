package models

// ChatMessageRequest gateway 从 WebSocket 收到的 AI 聊天消息。
type ChatMessageRequest struct {
	ConversationID string `json:"conversation_id,omitempty"` // AI 会话 ID；为空时 gateway 生成
	Text           string `json:"text"`
	MessageID      string `json:"message_id,omitempty"`
	MessageType    string `json:"message_type,omitempty"`
}

// IncomingMessage 客户端发送的消息外层。
type IncomingMessage struct {
	Type    string             `json:"type"`
	Content ChatMessageRequest `json:"content"`
}
