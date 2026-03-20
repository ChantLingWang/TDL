package chatconst

// MessageType 消息内容类型
const (
	// MessageTypeText 文本消息
	MessageTypeText = "text"
	// MessageTypeImage 图片消息
	MessageTypeImage = "image"
	// MessageTypeFile 文件消息
	MessageTypeFile = "file"
	// MessageTypeVoice 语音消息
	MessageTypeVoice = "voice"
	// MessageTypeVideo 视频消息
	MessageTypeVideo = "video"
)

// ConversationType 会话类型
const (
	// ConversationTypePrivate 私聊
	ConversationTypePrivate = "private"
	// ConversationTypeGroup 群聊
	ConversationTypeGroup = "group"
)
