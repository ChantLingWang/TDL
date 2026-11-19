package services

import (
	"context"
	"log"
	"time"

	"chat_service/app/api/models"
	"chat_service/app/config"
	chatconst "chat_service/app/const"
	"chat_service/app/infrastructure/kafka"

	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
	"github.com/google/uuid"
)

// HandleChat 处理用户发往 AI 的消息：生成/复用 conversation_id，投递 Kafka。
// gateway 不保存消息，不写数据库。
func HandleChat(userInfo *UserInfo, content models.ChatMessageRequest) {
	if content.Text == "" {
		return
	}

	conversationID := content.ConversationID
	if conversationID == "" {
		conversationID = uuid.NewString()
	}

	messageID := content.MessageID
	if messageID == "" {
		messageID = uuid.NewString()
	}

	messageType := content.MessageType
	if messageType == "" {
		messageType = chatconst.MessageTypeText
	}

	msg := &chatv1.MessageSent{
		SenderId:       userInfo.UserID,
		TargetUserId:   config.AIUserID,
		ConversationId: conversationID,
		Content:        content.Text,
		MessageId:      messageID,
		MessageType:    messageType,
		TimestampMs:    time.Now().UnixMilli(),
	}

	if err := kafka.GetProducer().SendProtoEvent(
		context.Background(), "chant.chat.v1.MessageSent", conversationID, msg,
	); err != nil {
		log.Printf("发送 MessageSent 失败 user=%s conversation=%s err=%v",
			userInfo.UserID, conversationID, err)
	}
}
