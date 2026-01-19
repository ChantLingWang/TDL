package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"user_service/app/api/utils"
	"user_service/app/infrastructure/kafka"
	"user_service/app/infrastructure/kafka/model"
	"user_service/app/models"
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

// HandleUserKickEvent 处理 Kafka 收到的踢人事件
func HandleUserKickEvent(ctx context.Context, data json.RawMessage) error {
	var kickData model.KickUserData
	if err := json.Unmarshal(data, &kickData); err != nil {
		return fmt.Errorf("解析踢人数据失败: %v", err)
	}

	// 调用 WebSocket Hub 强制断开连接
	hub := utils.GetHub()
	success := hub.KickUser(kickData.UserID)

	if success {
		log.Printf("用户 %s 已成功被踢下线", kickData.UserID)
	} else {
		log.Printf("用户 %s 当前不在线，无需踢下线", kickData.UserID)
	}

	return nil
}

// HandleBroadcastMessageEvent 处理 Kafka 收到的广播消息事件
func HandleBroadcastMessageEvent(ctx context.Context, data json.RawMessage) error {
	var broadcastEvent models.BroadcastChatMessage
	if err := json.Unmarshal(data, &broadcastEvent); err != nil {
		return fmt.Errorf("解析广播消息失败: %v", err)
	}

	hub := utils.GetHub()
	count := 0

	// 遍历 ID 列表，尝试在本地发送
	for _, userID := range broadcastEvent.TargetUserIDs {
		if hub.BroadcastToUser(userID, broadcastEvent.Message) {
			count++
		}
	}

	if count > 0 {
		log.Printf("收到广播消息，已成功发送给本地 %d 个用户", count)
	}

	return nil
}

// SendMessageToUser 发送消息给用户（核心路由逻辑）
// 1. 先尝试在本地 Hub 查找并发送
// 2. 如果本地没找到，则发送到 Kafka，让其他节点尝试发送
func (s *MessageService) SendMessageToUser(ctx context.Context, userID string, message []byte) error {
	hub := utils.GetHub()

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
	// 这里的 EventType 最好也提取到常量中
	err := s.producer.SendEvent(ctx, "user.chat.message", userID, chatEvent)
	if err != nil {
		return fmt.Errorf("消息转发 Kafka 失败: %v", err)
	}

	log.Printf("用户 %s 不在本地，消息已转发至 Kafka", userID)
	return nil
}

// BroadcastToUsers 批量发送消息给用户（本地优先 + 批量 Kafka）
func (s *MessageService) BroadcastToUsers(ctx context.Context, userIDs []string, message []byte) error {
	hub := utils.GetHub()
	var remoteUserIDs []string

	// 1. 尝试本地发送
	for _, userID := range userIDs {
		// 如果本地发送失败（不在线），则加入远程列表
		if !hub.BroadcastToUser(userID, message) {
			remoteUserIDs = append(remoteUserIDs, userID)
		}
	}

	// 2. 如果所有人都已在本地处理完，直接返回
	if len(remoteUserIDs) == 0 {
		return nil
	}

	// 3. 将剩余用户打包发送到 Kafka
	broadcastEvent := models.BroadcastChatMessage{
		TargetUserIDs: remoteUserIDs,
		Message:       message,
	}

	// 发送 "user.chat.broadcast" 事件
	// 使用随机 Key 或固定 Key，因为这是一个广播包，所有消费者都会收到（只要 GroupID 不同）
	// 注意：为了负载均衡，可以用 UUID 作为 Key
	err := s.producer.SendEvent(ctx, "user.chat.broadcast", "broadcast", broadcastEvent)
	if err != nil {
		return fmt.Errorf("广播消息转发 Kafka 失败: %v", err)
	}

	log.Printf("已将 %d 个用户的消息转发至 Kafka 广播", len(remoteUserIDs))
	return nil
}

// HandleChatMessageEvent 处理 Kafka 收到的聊天消息事件
// 这是其他节点发现用户不在它们那里，转发过来的消息
func HandleChatMessageEvent(ctx context.Context, data json.RawMessage) error {
	var chatEvent models.DistributedChatMessage
	if err := json.Unmarshal(data, &chatEvent); err != nil {
		return fmt.Errorf("解析聊天消息失败: %v", err)
	}

	hub := utils.GetHub()

	// 尝试在本地发送
	// 注意：如果这里还是没找到，说明用户可能真的离线了，或者在第三个节点
	if hub.BroadcastToUser(chatEvent.TargetUserID, chatEvent.Message) {
		log.Printf("收到 Kafka 转发消息，已成功发送给本地用户: %s", chatEvent.TargetUserID)
	} else {
		// 忽略，说明用户也不在这个节点
	}

	return nil
}
