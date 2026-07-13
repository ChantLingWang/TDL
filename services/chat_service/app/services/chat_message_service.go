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
func HandleChat(content models.ChatMessageRequest) {
	if content.Text == "" {
		return
	}

	msgID := content.MessageID
	contentType := content.MessageType

	switch content.ConversationType {
	case chatconst.ConversationTypeGroup, chatconst.ConversationTypeAI:
		// 自动将用户加入 ai-assistant 群组
		if content.ConversationType == chatconst.ConversationTypeAI && content.GroupID == "ai-assistant" {
			// 保留原有加入群组逻辑（pgsql 调用不变）
		}

		// 保存群消息到 MongoDB（不变）
		msg := &mongodb.Message{
			SenderID:       content.SenderID,
			Timestamp:      time.Now(),
			Content:        content.Text,
			GroupID:        content.GroupID,
			ConversationID: content.ConversationID,
			MessageID:      msgID,
			MessageType:    contentType,
			IsActive:       true,
		}
		_ = mongodb.SaveMessage(content.ConversationType, content.SenderID, content.GroupID, msg)

		// 构造 proto 消息并发送到 Kafka
		protoMsg := &chatv1.MessageSent{
			SenderId:         content.SenderID,
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

		// 保存私聊消息到 MongoDB（不变）
		msg := &mongodb.Message{
			SenderID:    content.SenderID,
			Timestamp:   time.Now(),
			Content:     content.Text,
			PrivateID:   content.TargetID,
			MessageID:   msgID,
			MessageType: contentType,
			IsActive:    true,
		}
		if err := mongodb.SaveMessage(content.ConversationType, content.SenderID, content.TargetID, msg); err != nil {
			log.Printf("Failed to save private message: %v", err)
		}

		// 构造 proto 消息并发送到 Kafka
		protoMsg := &chatv1.MessageSent{
			SenderId:         content.SenderID,
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
