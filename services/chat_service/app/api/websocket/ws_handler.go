package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"chat_service/app/infrastructure/grpc"
	"chat_service/app/infrastructure/kafka"
	"chat_service/app/services"

	"github.com/gin-gonic/gin"
)

// IncomingMessage 定义客户端发送的消息格式
type IncomingMessage struct {
	Type    string          `json:"type"`    // 消息类型: "private_chat", "group_chat", "ping" 等
	Content json.RawMessage `json:"content"` // 消息内容，根据类型不同而结构不同
}

// HandleWebSocket 处理 WebSocket 连接请求
func HandleWebSocket(c *gin.Context) {
	// 1. 从 Header 获取 Token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
		return
	}

	// 解析 Token (Bearer xxx)
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
		return
	}

	// 2. 通过 gRPC 调用 auth_service 验证 Token
	authClient := grpc.GetAuthClient()
	resp, err := authClient.VerifyToken(context.Background(), token)
	if err != nil || !resp.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": resp.Message})
		return
	}

	// 3. 从响应中获取完整的用户信息
	userInfo := &services.UserInfo{
		UserID:   resp.UserId,
		Username: resp.Username,
		Email:    resp.Email,
	}
	log.Printf("WebSocket connection validated for user: %s (%s)", userInfo.Username, userInfo.UserID)

	// 4. 升级 HTTP 连接为 WebSocket
	conn, err := services.WSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket: %v", err)
		return
	}

	// 3. 初始化连接包装器
	wsConn := services.NewWSConnection(conn)

	// 4. 获取 Hub 单例
	hub := services.GetWSHub()

	// 5. 创建客户端实例
	client := services.NewWSClient(hub, wsConn, userInfo)

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
		case kafka.WSMsgTypeChat:
			services.HandleChat(userInfo.UserID, msg.Content)
		case kafka.WSMsgTypePing:
			// 心跳包处理
			log.Printf("Received ping from %s", userInfo.UserID)
		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}

		return nil
	})

	// 9. 连接断开后的清理工作
	// ReadLoop 返回意味着连接已关闭
	hub.Unregister(client)
}
