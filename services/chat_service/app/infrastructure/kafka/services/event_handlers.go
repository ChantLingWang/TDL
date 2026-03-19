package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"chat_service/app/models"
)

// KafkaProducerIface Kafka 生产者接口
type KafkaProducerIface interface {
	SendEvent(ctx context.Context, eventType string, key string, data interface{}) error
}

var (
	// 全局生产者实例
	globalProducer KafkaProducerIface
)

// SetProducer 设置全局生产者实例（由 main.go 调用）
func SetProducer(producer KafkaProducerIface) {
	globalProducer = producer
}

// HandleUserKickEvent 处理 Kafka 收到的踢人事件
func HandleUserKickEvent(ctx context.Context, data json.RawMessage) error {
	var kickData models.KickUserData
	if err := json.Unmarshal(data, &kickData); err != nil {
		return fmt.Errorf("解析踢人数据失败: %v", err)
	}

	log.Printf("用户 %s 已被踢出", kickData.UserID)
	return nil
}

// HandleBroadcastMessageEvent 处理 Kafka 收到的广播消息事件
func HandleBroadcastMessageEvent(ctx context.Context, data json.RawMessage) error {
	log.Printf("收到广播消息但未设置处理器")
	return nil
}

// HandleChatMessageEvent 处理 Kafka 收到的聊天消息事件
func HandleChatMessageEvent(ctx context.Context, data json.RawMessage) error {
	log.Printf("收到聊天消息但未设置处理器")
	return nil
}

// HandleUserRegisteredEvent 处理用户注册事件（暂时为空实现）
func HandleUserRegisteredEvent(ctx context.Context, data json.RawMessage) error {
	log.Printf("收到用户注册事件，但暂未实现处理逻辑")
	return nil
}
