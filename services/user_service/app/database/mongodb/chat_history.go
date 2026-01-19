package mongodb

import (
	"fmt"
	"user_service/app/infrastructure/kafka"
)

// SaveMessage 统一保存消息入口
// conversationType: "group" 或 "private"
func SaveMessage(conversationType string, senderID, targetID string, msg *Message) error {
	switch conversationType {
	case kafka.ConversationTypeGroup:
		// 群聊：targetID 即为 GroupID
		return GetGroupMessageHistoryService().AddGroupMessageByUser(targetID, msg)
	case kafka.ConversationTypePrivate:
		// 私聊：需要 senderID 和 targetID 来生成 SessionID
		return GetPrivateMessageHistoryService().AddPrivateMessage(senderID, targetID, msg)
	default:
		return fmt.Errorf("unsupported conversation type: %s", conversationType)
	}
}
