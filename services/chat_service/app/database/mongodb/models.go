package mongodb

import (
	"time"
)

// Message 消息结构体（群聊和私聊共用）
type Message struct {
	SenderID    string    `bson:"sender_id"`    // 发送者ID
	Timestamp   time.Time `bson:"timestamp"`    // 时间戳
	Content     string    `bson:"content"`      // 消息内容
	PrivateID   string    `bson:"private_id"`   // 私聊接收者ID（仅私聊时使用）
	GroupID     string    `bson:"group_id"`     // 群ID（仅群聊时使用）
	MessageID   string    `bson:"message_id"`   // 消息ID
	MessageType string    `bson:"message_type"` // 消息类型
	IsActive    bool      `bson:"is_active"`    // 是否可见
	Read        bool      `bson:"read"`         // 是否已读
}

// GroupMessageHistory 表示群聊聊天记录的结构
type GroupMessageHistory struct {
	GroupID        string    `bson:"group_id"`        // 群组ID
	DateIdentifier string    `bson:"date_identifier"` // 日期标识符
	Messages       []Message `bson:"messages"`        // 消息数组
	Count          int       `bson:"count"`           // 当前文档消息数量，用于分桶控制
	StartTime      time.Time `bson:"start_time"`      // 桶内第一条消息时间
	EndTime        time.Time `bson:"end_time"`        // 桶内最后一条消息时间
}

// PrivateMessageHistory 表示私聊记录的结构
type PrivateMessageHistory struct {
	SessionID      string    `bson:"session_id"`      // 会话ID (两个用户ID排序后的组合，确保唯一性)
	DateIdentifier string    `bson:"date_identifier"` // 日期标识符
	Messages       []Message `bson:"messages"`        // 消息数组
}
