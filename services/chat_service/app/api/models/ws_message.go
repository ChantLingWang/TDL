package models

// ChatPart 前端发来的一段内容：text / image / audio / video。
// data 是 base64 字符串，encoding/json 会解码成 bytes。
type ChatPart struct {
	Type     string                 `json:"type"`
	Text     string                 `json:"text,omitempty"`
	Data     []byte                 `json:"data,omitempty"`
	MimeType string                 `json:"mime_type,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ChatMessageRequest gateway 从 WebSocket 收到的 AI 聊天消息。
type ChatMessageRequest struct {
	ConversationID string     `json:"conversation_id,omitempty"` // AI 会话 ID；为空时 gateway 生成
	Text           string     `json:"text,omitempty"`            // 兼容旧客户端；优先用 parts
	Parts          []ChatPart `json:"parts,omitempty"`
	MessageID      string     `json:"message_id,omitempty"`
	MessageType    string     `json:"message_type,omitempty"`
}

// IncomingMessage 客户端发送的消息外层。
type IncomingMessage struct {
	Type    string             `json:"type"`
	Content ChatMessageRequest `json:"content"`
}
