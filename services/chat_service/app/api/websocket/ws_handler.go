package websocket

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"chat_service/app/api/models"
	"chat_service/app/infrastructure/grpc"
	"chat_service/app/infrastructure/kafka"
	"chat_service/app/services"

	"github.com/gin-gonic/gin"
)

// HandleWebSocket 处理 WebSocket 连接请求
func HandleWebSocket(c *gin.Context) {
	// 从 Context 获取用户信息
	userInfo := c.MustGet("userInfo").(*services.UserInfo)

	// 1. 升级 HTTP 连接为 WebSocket
	conn, err := services.WSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket: %v", err)
		return
	}

	log.Printf("WS upgrade OK for user %s", userInfo.UserID)

	// 3. 初始化连接包装器
	wsConn := services.NewWSConnection(conn)

	// 4. 获取 Hub 单例
	hub := services.GetWSHub()

	// 5. 创建客户端实例
	client := services.NewWSClient(hub, wsConn, userInfo)

	// 6. 注册到 Hub
	hub.Register(client)

	// 7. 启动写泵 (WritePump) - 负责下行消息（必须在发送消息之前启动）
	go client.WritePump()

	// 8. 发送欢迎消息
	welcomeMsg := `{"type":"system","content":{"message":"Connected to chat-local"}}`
	client.Send <- []byte(welcomeMsg)
	log.Printf("WS welcome sent to %s", userInfo.UserID)

	// 9. 启动读泵 (ReadLoop) - 负责上行消息
	// 这里的匿名函数就是 "Callback"（回调函数），ReadLoop 每收到一条消息，就会调用它一次
	wsConn.ReadLoop(func(messageType int, data []byte) error {
		var msg models.IncomingMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("Invalid message format: %v", err)
			return nil // 格式错误忽略，不断开连接
		}

		// 根据消息类型分发给不同的处理函数
		switch msg.Type {
		case kafka.WSMsgTypeChat:
			services.HandleChat(msg.Content)
		case kafka.WSMsgTypePing:
			// 心跳包处理
			log.Printf("Received ping from %s", userInfo.UserID)
		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}

		return nil
	})

	// 10. 连接断开后的清理工作
	// ReadLoop 返回意味着连接已关闭
	log.Printf("WS connection closed for user %s", userInfo.UserID)
	hub.Unregister(client)

	// 11. 用户离线，通过 gRPC 更新用户最后离线时间
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client := grpc.GetLastOfflineTimeClient()
		resp, err := client.UpdateLastOfflineTime(ctx, userInfo.UserID)
		if err != nil {
			log.Printf("Failed to update last offline time for user %s: %v", userInfo.UserID, err)
			return
		}
		if !resp.Success {
			log.Printf("Failed to update last offline time for user %s: success=false", userInfo.UserID)
			return
		}
	}()
}
