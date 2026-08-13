package services

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"chat_service/app/api/models"
	chatconst "chat_service/app/const"
	"chat_service/app/database/mongodb"
	"chat_service/app/database/pgsql"
	"chat_service/app/infrastructure/kafka"

	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
)

// getGroupPartitionKey 根据 GroupID 数字部分实现奇偶分区
func getGroupPartitionKey(groupID string) string {
	numStr := strings.TrimPrefix(groupID, "G")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return "0"
	}
	return strconv.Itoa(num % 2)
}

// HandleChat 处理统一聊天逻辑，使用 proto MessageSent。
// userID 为认证中间件从 JWT 中解析出的登录用户 ID，
// 消息的 sender 一律以登录态为准，忽略客户端传来的 sender_id。
func HandleChat(userID string, content models.ChatMessageRequest) {
	if content.Text == "" {
		return
	}

	msgID := content.MessageID
	contentType := content.MessageType
	senderID := userID

	switch content.ConversationType {
	case chatconst.ConversationTypeGroup, chatconst.ConversationTypeAI, chatconst.ConversationTypeAIResearch:
		// 群聊 / AI 会话必须先校验当前用户是群成员，防止向任意群注入消息
		isMember, err := pgsql.NewUserGroupService(pgsql.GetDBManager()).
			IsUserInGroup(senderID, content.GroupID)
		if err != nil || !isMember {
			log.Printf("拒绝发送群消息：用户不是群成员 user=%s group=%s err=%v",
				senderID, content.GroupID, err)
			return
		}

		// 保存群消息到 MongoDB
		msg := &mongodb.Message{
			SenderID:       senderID,
			Timestamp:      time.Now(),
			Content:        content.Text,
			GroupID:        content.GroupID,
			ConversationID: content.ConversationID,
			MessageID:      msgID,
			MessageType:    contentType,
			IsActive:       true,
		}
		if err := mongodb.SaveMessage(content.ConversationType, senderID, content.GroupID, msg); err != nil {
			log.Printf("Failed to save group message: %v", err)
		}

		// 构造 proto 消息并发送到 Kafka
		protoMsg := &chatv1.MessageSent{
			SenderId:         senderID,
			GroupId:          content.GroupID,
			Content:          content.Text,
			MessageId:        msgID,
			MessageType:      contentType,
			ConversationType: content.ConversationType,
		}
		partitionKey := getGroupPartitionKey(content.GroupID)
		kafka.GetProducer().SendProtoEvent(context.Background(), "chant.chat.v1.MessageSent", partitionKey, protoMsg)
		return

	case chatconst.ConversationTypePrivate:
		if content.TargetID == "" {
			log.Println("Invalid private chat: TargetID is empty")
			return
		}
		if content.TargetID == senderID {
			log.Println("Invalid private chat: cannot message self")
			return
		}

		// 保存私聊消息到 MongoDB
		msg := &mongodb.Message{
			SenderID:    senderID,
			Timestamp:   time.Now(),
			Content:     content.Text,
			PrivateID:   content.TargetID,
			MessageID:   msgID,
			MessageType: contentType,
			IsActive:    true,
		}
		if err := mongodb.SaveMessage(content.ConversationType, senderID, content.TargetID, msg); err != nil {
			log.Printf("Failed to save private message: %v", err)
		}

		// 构造 proto 消息并发送到 Kafka
		protoMsg := &chatv1.MessageSent{
			SenderId:         senderID,
			TargetUserId:     content.TargetID,
			Content:          content.Text,
			MessageId:        msgID,
			MessageType:      contentType,
			ConversationType: content.ConversationType,
		}
		kafka.GetProducer().SendProtoEvent(context.Background(), "chant.chat.v1.MessageSent", content.TargetID, protoMsg)
		return

	default:
		log.Printf("Unknown conversation type: %s", content.ConversationType)
		return
	}
}
