package models

// ChatMessageRequest 聊天消息请求结构
type ChatMessageRequest struct {
	ConversationID   string `json:"conversation_id,omitempty"` // AI 会话 ID
	ConversationType string `json:"conversation_type"`         // 会话类型: "private", "group"
	TargetID         string `json:"target_id,omitempty"`       // 私聊接收者
	GroupID          string `json:"group_id,omitempty"`        // 群聊接收者
	Text             string `json:"text"`                      // 文本内容
	MessageID        string `json:"message_id,omitempty"`      // 消息ID,方便溯源,去重
	MessageType      string `json:"message_type,omitempty"`    // 消息类型: "text", "image" 等
}

// IncomingMessage 定义客户端发送的消息格式
type IncomingMessage struct {
	Type    string             `json:"type"`
	Content ChatMessageRequest `json:"content"` // 消息内容，根据类型不同而结构不同
}
