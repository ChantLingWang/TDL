package websocket

import (
	"encoding/json"
	"log"

	"chat_service/app/infrastructure/kafka"
	"chat_service/app/services"

	"github.com/gin-gonic/gin"
)

// IncomingMessage 定义客户端发送的消息格式
type IncomingMessage struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"` // 消息内容，根据类型不同而结构不同
}

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

	// 10. 用户离线，更新所有会话的最后阅读时间
	// TODO: 需要查询用户所在的所有群和私聊，然后更新每个会话的 LastReadTime
	// 当前简化处理：下次用户上线时，会自动获取该时间之后的消息
	_ = userInfo.UserID // 用户ID，用于后续查询
}
