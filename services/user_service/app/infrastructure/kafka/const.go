package kafka

const (
	// EventUserRegistered 用户注册事件
	EventUserRegistered = "user.registered"
	// EventUserKick 用户强制下线事件
	EventUserKick = "user.kick"
	// EventUserChatMessage 用户聊天消息事件
	EventUserChatMessage = "user.chat.message"
	// EventUserBroadcastMessage 用户广播消息事件
	EventUserBroadcastMessage = "user.chat.broadcast"

	// WebSocket 消息类型
	// WSMsgTypeChat 统一聊天消息
	WSMsgTypeChat = "chat"
	// WSMsgTypePrivateChat 私聊消息
	WSMsgTypePrivateChat = "private_chat"
	// WSMsgTypeGroupChat 群聊消息
	WSMsgTypeGroupChat = "group_chat"
	// WSMsgTypePing 心跳消息
	WSMsgTypePing = "ping"

	// 会话类型
	// ConversationTypePrivate 私聊
	ConversationTypePrivate = "private"
	// ConversationTypeGroup 群聊
	ConversationTypeGroup = "group"
)
