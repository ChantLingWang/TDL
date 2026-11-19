package websocket

import (
	"encoding/json"
	"log"

	"chat_service/app/api/models"
	"chat_service/app/infrastructure/kafka"
	"chat_service/app/services"

	"github.com/gin-gonic/gin"
)

// HandleWebSocket 处理 WebSocket 连接：鉴权后只负责收发，不落库。
func HandleWebSocket(c *gin.Context) {
	userInfo := c.MustGet("userInfo").(*services.UserInfo)

	conn, err := services.WSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket: %v", err)
		return
	}
	conn.SetReadLimit(64 * 1024)

	wsConn := services.NewWSConnection(conn)
	hub := services.GetWSHub()
	client := services.NewWSClient(hub, wsConn, userInfo)
	hub.Register(client)

	go client.WritePump()

	client.Send <- []byte(`{"type":"system","content":{"message":"Connected to chat gateway"}}`)

	wsConn.ReadLoop(func(messageType int, data []byte) error {
		var msg models.IncomingMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("Invalid message format: %v", err)
			return nil
		}

		switch msg.Type {
		case kafka.WSMsgTypeChat:
			services.HandleChat(userInfo, msg.Content)
		case kafka.WSMsgTypePing:
			log.Printf("Received ping from %s", userInfo.UserID)
		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
		return nil
	})

	log.Printf("WS connection closed for user %s", userInfo.UserID)
	hub.Unregister(client)
}
