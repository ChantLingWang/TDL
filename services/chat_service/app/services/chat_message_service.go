package services

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"chat_service/app/api/models"
	chatconst "chat_service/app/const"
	"chat_service/app/database/mongodb"
	"chat_service/app/infrastructure/kafka"
)

// getGroupPartitionKey 根据 GroupID 数字部分实现奇偶分区
// G1 -> "0", G2 -> "1", G3 -> "0", G4 -> "1"...
func getGroupPartitionKey(groupID string) string {
	// 提取 GroupID 中的数字部分（G1 -> 1, G2 -> 2）
	numStr := strings.TrimPrefix(groupID, "G")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return "0" // 默认分区 0
	}
	// 奇数 -> 0 区，偶数 -> 1 区
	return strconv.Itoa(num % 2)
}

// GroupChatMessage 群聊消息结构
type GroupChatMessage struct {
	GroupID     string `json:"group_id"`
	SenderID    string `json:"sender_id"`
	Content     string `json:"content"`
	Timestamp   int64  `json:"timestamp"`
	MessageID   string `json:"message_id"`
	MessageType string `json:"message_type,omitempty"`
}

// PrivateChatMessage 私聊消息结构
type PrivateChatMessage struct {
	SenderID     string `json:"sender_id"`
	TargetUserID string `json:"target_user_id"`
	Content      string `json:"content"`
	Timestamp    int64  `json:"timestamp"`
	MessageID    string `json:"message_id"`
	MessageType  string `json:"message_type,omitempty"`
}

// ToJSON 转换为 JSON 字节数组
func (g *GroupChatMessage) ToJSON() []byte {
	data, _ := json.Marshal(g)
	return data
}

// ToJSON 转换为 JSON 字节数组
func (p *PrivateChatMessage) ToJSON() []byte {
	data, _ := json.Marshal(p)
	return data
}

// HandleChat 处理统一聊天逻辑
func HandleChat(content models.ChatMessageRequest) {
	if content.Text == "" {
		return
	}

	msgID := content.MessageID
	contentType := content.MessageType

	switch content.ConversationType {
	case chatconst.ConversationTypeGroup:
		// 群聊逻辑
		// 先保存群消息到数据库
		msg := &mongodb.Message{
			SenderID:    content.SenderID,
			Timestamp:   time.Now(),
			Content:     content.Text,
			GroupID:     content.GroupID,
			MessageID:   msgID,
			MessageType: contentType,
			IsActive:    true,
		}
		_ = mongodb.SaveMessage(content.ConversationType, content.SenderID, content.GroupID, msg)

		// 再发送群消息到 Kafka
		groupMsg := &GroupChatMessage{
			GroupID:     content.GroupID,
			SenderID:    content.SenderID,
			Content:     content.Text,
			Timestamp:   time.Now().UnixMilli(),
			MessageID:   msgID,
			MessageType: contentType,
		}
		// 使用 GroupID 数字部分 % 分区数 实现奇偶分区
		partitionKey := getGroupPartitionKey(content.GroupID)
		kafka.GetProducer().SendEvent(context.Background(), kafka.EventGroupChatMessage, msgID, partitionKey, groupMsg.ToJSON())
		return

	case chatconst.ConversationTypePrivate:
		if content.TargetID == "" {
			log.Println("Invalid private chat: TargetID is empty")
			return
		}

		// 保存私聊消息到数据库
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

		// 构造私聊消息（给消费者推送给目标用户）
		privateMsg := &PrivateChatMessage{
			SenderID:    content.SenderID,
			TargetUserID: content.TargetID,
			Content:     content.Text,
			Timestamp:   time.Now().UnixMilli(),
			MessageID:   msgID,
			MessageType: contentType,
		}

		// 发送私聊消息到 Kafka（各实例消费后推送给各自本地的在线目标用户）
		// 使用 TargetUserID 作为 Key，保证同一用户的私聊消息有序
		kafka.GetProducer().SendEvent(context.Background(), kafka.EventPrivateChatMessage, msgID, content.TargetID, privateMsg.ToJSON())
		return

	default:
		log.Printf("Unknown conversation type: %s", content.ConversationType)
		return
	}
}
