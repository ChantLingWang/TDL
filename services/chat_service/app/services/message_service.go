package services

import (
	"context"
	"fmt"
	"log"

	"chat_service/app/infrastructure/kafka"
	"chat_service/app/models"

	"github.com/google/uuid"
)

// MessageService 负责处理消息路由及业务逻辑
type MessageService struct {
	producer *kafka.KafkaProducer
}

var (
	// 单例实例
	messageServiceInstance *MessageService
)

// InitMessageService 初始化 MessageService
func InitMessageService(producer *kafka.KafkaProducer) {
	messageServiceInstance = &MessageService{
		producer: producer,
	}
}

// GetMessageService 获取单例
func GetMessageService() *MessageService {
	return messageServiceInstance
}

// SendMessageToUser 发送消息给用户（核心路由逻辑）
// 1. 先尝试在本地 Hub 查找并发送
// 2. 如果本地没找到，则发送到 Kafka，让其他节点尝试发送
func (s *MessageService) SendMessageToUser(ctx context.Context, userID string, message []byte) error {
	hub := GetWSHub()

	// 1. 尝试本地发送
	if hub.IsUserOnline(userID) {
		success := hub.BroadcastToUser(userID, message)
		if success {
			return nil
		}
	}

	// 2. 本地不在或发送失败，转发到 Kafka
	chatEvent := models.DistributedChatMessage{
		TargetUserID: userID,
		Message:      message,
	}

	// 发送 "user.chat.message" 事件
	// 使用 TargetUserID 作为 Key，保证消息顺序
	// 使用 uuid 作为 messageID
	err := s.producer.SendEvent(ctx, "user.chat.message", uuid.New().String(), userID, chatEvent)
	if err != nil {
		return fmt.Errorf("消息转发 Kafka 失败: %v", err)
	}

	log.Printf("用户 %s 不在本地，消息已转发至 Kafka", userID)
	return nil
}
