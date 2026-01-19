package v1

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"user_service/app/api/utils"
	"user_service/app/database/mongodb"
	"user_service/app/database/pgsql"
	"user_service/app/infrastructure/kafka"
	"user_service/app/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// IncomingMessage 定义客户端发送的消息格式
type IncomingMessage struct {
	Type    string          `json:"type"`    // 消息类型: "private_chat", "group_chat", "ping" 等
	Content json.RawMessage `json:"content"` // 消息内容，根据类型不同而结构不同
}

// UnifiedChatContent 统一聊天消息内容结构
type UnifiedChatContent struct {
	ConversationType string `json:"conversation_type"`   // 会话类型: "private", "group"
	TargetID         string `json:"target_id,omitempty"` // 私聊接收者
	GroupID          string `json:"group_id,omitempty"`  // 群聊接收者
	Text             string `json:"text"`                // 文本内容
}

// ConnectWebSocket 处理 WebSocket 连接请求
func ConnectWebSocket(c *gin.Context) {
	// 1. 获取用户ID
	userID := c.Query("user_id") // 从token中获取 user_id
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	// 2. 升级 HTTP 连接为 WebSocket
	conn, err := utils.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket: %v", err)
		return
	}

	// 3. 初始化连接包装器
	wsConn := utils.NewWSConnection(conn)

	// 4. 获取 Hub 单例
	hub := utils.GetHub()

	// 5. 创建客户端实例
	client := utils.NewClient(hub, wsConn, userID)

	// 6. 注册到 Hub
	hub.Register(client)

	// 7. 启动写泵 (WritePump) - 负责下行消息
	go client.WritePump()

	// 8. 启动读泵 (ReadLoop) - 负责上行消息
	// 这里的匿名函数就是 "Callback"（回调函数），ReadLoop 每收到一条消息，就会调用它一次
	wsConn.ReadLoop(func(messageType int, data []byte) error {
		var msg IncomingMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("Invalid message format: %v", err)
			return nil // 格式错误忽略，不断开连接
		}

		// 根据消息类型分发给不同的处理函数
		switch msg.Type {
		case kafka.WSMsgTypeChat: // 统一为 chat 类型
			handleChat(userID, msg.Content)
		case kafka.WSMsgTypePing:
			// 心跳包处理
			log.Printf("Received ping from %s", userID)
		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}

		return nil
	})

	// 9. 连接断开后的清理工作
	// ReadLoop 返回意味着连接已关闭
	hub.Unregister(client)
}

// handleChat 处理统一聊天逻辑
func handleChat(senderID string, content json.RawMessage) {
	var chatContent UnifiedChatContent
	if err := json.Unmarshal(content, &chatContent); err != nil {
		log.Printf("Invalid chat content: %v", err)
		return
	}

	if chatContent.Text == "" {
		return
	}

	// 兼容逻辑：如果没有传 ConversationType，尝试通过 ID 字段推断
	if chatContent.ConversationType == "" {
		if chatContent.GroupID != "" {
			chatContent.ConversationType = kafka.ConversationTypeGroup
		} else if chatContent.TargetID != "" {
			chatContent.ConversationType = kafka.ConversationTypePrivate
		}
	}

	// 1. 构建消息对象
	msg := &mongodb.Message{
		Timestamp:   time.Now(),
		Content:     chatContent.Text,
		UserID:      senderID,
		Username:    senderID, // 暂时用 ID 代替用户名，实际应查询 User 服务
		MessageID:   uuid.New().String(),
		MessageType: "text",
		IsActive:    true,
	}

	var targetUserIDs []string
	var msgType string
	var conversationID string

	// 2. 判断是私聊还是群聊
	switch chatContent.ConversationType {
	case kafka.ConversationTypeGroup:
		if chatContent.GroupID == "" {
			log.Println("Invalid group chat: GroupID is empty")
			return
		}
		// 群聊逻辑
		msgType = kafka.WSMsgTypeGroupChat
		conversationID = chatContent.GroupID

		// 获取群成员
		userGroupService := pgsql.NewUserGroupService(pgsql.GetDBManager())
		members, err := userGroupService.GetGroupMembers(chatContent.GroupID)
		if err != nil {
			log.Printf("Failed to get group members: %v", err)
			return
		}
		targetUserIDs = members

	case kafka.ConversationTypePrivate:
		if chatContent.TargetID == "" {
			log.Println("Invalid private chat: TargetID is empty")
			return
		}
		// 私聊逻辑
		msgType = kafka.WSMsgTypePrivateChat
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
		"type":            msgType,
		"conversation_id": conversationID,
		"sender":          senderID,
		"content":         chatContent.Text,
		"time":            msg.Timestamp,
	}

	if msgType == kafka.WSMsgTypeGroupChat {
		responseMsg["group_id"] = chatContent.GroupID
	}

	msgBytes, _ := json.Marshal(responseMsg)

	// 4. 批量广播给目标用户
	messageService := services.GetMessageService()
	if messageService != nil {
		// 使用 BroadcastToUsers 替代循环 SendMessageToUser
		// 这样可以实现本地直发，异地打包广播
		err := messageService.BroadcastToUsers(context.Background(), targetUserIDs, msgBytes)
		if err != nil {
			log.Printf("Failed to broadcast message: %v", err)
		}
	} else {
		log.Println("MessageService not initialized")
	}
}
