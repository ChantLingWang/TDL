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

	// 群消息本地广播回调函数（由 main.go 注入）
	groupMessageLocalBroadcast func(groupID string, message []byte)

	// 私聊消息本地广播回调函数（由 main.go 注入）
	privateMessageLocalBroadcast func(targetUserID string, message []byte)
)

// SetProducer 设置全局生产者实例（由 main.go 调用）
func SetProducer(producer KafkaProducerIface) {
	globalProducer = producer
}

// RegisterGroupMessageLocalBroadcast 注册群消息本地广播函数（由 main.go 调用）
// 回调函数负责获取群成员并推送给本地在线用户
func RegisterGroupMessageLocalBroadcast(fn func(groupID string, message []byte)) {
	groupMessageLocalBroadcast = fn
}

// RegisterPrivateMessageLocalBroadcast 注册私聊消息本地广播函数（由 main.go 调用）
// 回调函数负责推送给本地在线用户
func RegisterPrivateMessageLocalBroadcast(fn func(targetUserID string, message []byte)) {
	privateMessageLocalBroadcast = fn
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

// HandleGroupChatMessageEvent 处理 Kafka 收到的群聊消息事件
func HandleGroupChatMessageEvent(ctx context.Context, data json.RawMessage) error {
	// 1. 解析群消息
	type GroupChatMessage struct {
		GroupID     string `json:"group_id"`
		SenderID    string `json:"sender_id"`
		Content     string `json:"content"`
		Timestamp   int64  `json:"timestamp"`
		MessageID   string `json:"message_id"`
		MessageType string `json:"message_type,omitempty"`
	}

	var groupMsg GroupChatMessage
	if err := json.Unmarshal(data, &groupMsg); err != nil {
		return fmt.Errorf("解析群消息失败: %v", err)
	}

	log.Printf("收到群消息: group_id=%s, sender=%s, content=%s", groupMsg.GroupID, groupMsg.SenderID, groupMsg.Content)

	// 2. 构造前端消息格式
	responseMsg := map[string]interface{}{
		"type":            "group_chat",
		"conversation_id": groupMsg.GroupID,
		"group_id":        groupMsg.GroupID,
		"sender":          groupMsg.SenderID,
		"content":         groupMsg.Content,
		"time":            groupMsg.Timestamp,
	}

	msgBytes, _ := json.Marshal(responseMsg)

	// 3. 调用注册的回调函数，推送给本地在线用户
	if groupMessageLocalBroadcast != nil {
		groupMessageLocalBroadcast(groupMsg.GroupID, msgBytes)
		log.Printf("群消息已推送给本地在线用户: group_id=%s", groupMsg.GroupID)
	} else {
		log.Printf("警告：未注册群消息本地广播函数")
	}

	return nil
}

// HandlePrivateChatMessageEvent 处理 Kafka 收到的私聊消息事件
func HandlePrivateChatMessageEvent(ctx context.Context, data json.RawMessage) error {
	// 1. 解析私聊消息
	type PrivateChatMessage struct {
		SenderID     string `json:"sender_id"`
		TargetUserID string `json:"target_user_id"`
		Content      string `json:"content"`
		Timestamp    int64  `json:"timestamp"`
		MessageID    string `json:"message_id"`
		MessageType  string `json:"message_type,omitempty"`
	}

	var privateMsg PrivateChatMessage
	if err := json.Unmarshal(data, &privateMsg); err != nil {
		return fmt.Errorf("解析私聊消息失败: %v", err)
	}

	log.Printf("收到私聊消息: from=%s, to=%s, content=%s", privateMsg.SenderID, privateMsg.TargetUserID, privateMsg.Content)

	// 2. 构造前端消息格式
	responseMsg := map[string]interface{}{
		"type":            "private_chat",
		"conversation_id": privateMsg.TargetUserID,
		"sender":          privateMsg.SenderID,
		"content":         privateMsg.Content,
		"time":            privateMsg.Timestamp,
	}

	msgBytes, _ := json.Marshal(responseMsg)

	// 3. 调用注册的回调函数，推送给本地在线用户
	if privateMessageLocalBroadcast != nil {
		privateMessageLocalBroadcast(privateMsg.TargetUserID, msgBytes)
		log.Printf("私聊消息已推送给本地在线用户: to=%s", privateMsg.TargetUserID)
	} else {
		log.Printf("警告：未注册私聊消息本地广播函数")
	}

	return nil
}
