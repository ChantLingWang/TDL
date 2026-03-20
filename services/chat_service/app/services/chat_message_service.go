package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	chatconst "chat_service/app/const"
	"chat_service/app/database/mongodb"
	"chat_service/app/database/pgsql"
	"chat_service/app/infrastructure/kafka"
)

// SendMessageFunc 发送消息函数的类型
type SendMessageFunc func(ctx context.Context, userIDs []string, message []byte)

// 全局发送消息函数
var sendMessageFn SendMessageFunc

// RegisterSendMessageFunc 注册发送消息函数（由 main.go 调用）
func RegisterSendMessageFunc(fn SendMessageFunc) {
	sendMessageFn = fn
}

// ChatMessageRequest 聊天消息请求结构
type ChatMessageRequest struct {
	ConversationType string `json:"conversation_type"`    // 会话类型: "private", "group"
	TargetID         string `json:"target_id,omitempty"`  // 私聊接收者
	GroupID          string `json:"group_id,omitempty"`   // 群聊接收者
	Text             string `json:"text"`                  // 文本内容
	MessageID		 string `json:"message_id,omitempty"`  // 消息ID,方便溯源,去重
	MessageType      string `json:"message_type,omitempty"` // 消息类型: "text", "image" 等
}

// HandleChat 处理统一聊天逻辑
func HandleChat(senderID string, content json.RawMessage) {
	var chatContent ChatMessageRequest
	if err := json.Unmarshal(content, &chatContent); err != nil {
		log.Printf("Invalid chat content: %v", err)
		return
	}

	if chatContent.Text == "" {
		return
	}

	// 1. 构建消息对象
	msgID := chatContent.MessageID
	contentType := chatContent.MessageType

	msg := &mongodb.Message{
		Timestamp:   time.Now(),
		Content:     chatContent.Text,
		TouserID:    senderID,
		MessageID:   msgID,
		MessageType: contentType,
		IsActive:    true,
	}

	var targetUserIDs []string
	var wsMsgType string
	var conversationID string

	// 2. 判断是私聊还是群聊
	switch chatContent.ConversationType {
	case chatconst.ConversationTypeGroup:
		// 群聊逻辑
		wsMsgType = kafka.WSMsgTypeGroupChat
		conversationID = chatContent.GroupID

		// 获取群成员
		userGroupService := pgsql.NewUserGroupService(pgsql.GetDBManager())
		members, err := userGroupService.GetGroupMembers(chatContent.GroupID)
		if err != nil {
			log.Printf("Failed to get group members: %v", err)
			return
		}
		targetUserIDs = members

	case chatconst.ConversationTypePrivate:
		if chatContent.TargetID == "" {
			log.Println("Invalid private chat: TargetID is empty")
			return
		}
		// 私聊逻辑
		wsMsgType = kafka.WSMsgTypePrivateChat
		conversationID = chatContent.TargetID

		// 目标用户就是接收者
		targetUserIDs = []string{chatContent.TargetID}

	default:
		log.Printf("Unknown conversation type: %s", chatContent.ConversationType)
		return
	}

	// 统一持久化消息
	if err := mongodb.SaveMessage(chatContent.ConversationType, senderID, conversationID, msg); err != nil {
		log.Printf("Failed to save message: %v", err)
		// 持久化失败是否阻断发送？通常建议继续发送，或者返回错误给前端
	}

	// 3. 构造发送给前端的消息
	// 保持结构清晰，统一返回格式
	responseMsg := map[string]interface{}{
		"type":            wsMsgType,
		"conversation_id": conversationID,
		"sender":          senderID,
		"content":         chatContent.Text,
		"time":            msg.Timestamp,
	}

	if wsMsgType == kafka.WSMsgTypeGroupChat {
		responseMsg["group_id"] = chatContent.GroupID
	}

	msgBytes, _ := json.Marshal(responseMsg)

	// 4. 批量广播给目标用户
	hub := GetWSHub()
	var remoteUserIDs []string

	// 尝试本地发送
	for _, userID := range targetUserIDs {
		if !hub.BroadcastToUser(userID, msgBytes) {
			remoteUserIDs = append(remoteUserIDs, userID)
		}
	}

	// 如果有远程用户，通过注册的发送函数发送到 Kafka
	if len(remoteUserIDs) > 0 && sendMessageFn != nil {
		sendMessageFn(context.Background(), remoteUserIDs, msgBytes)
	}
}
