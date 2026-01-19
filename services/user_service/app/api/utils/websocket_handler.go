package utils

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Upgrader 标准的WebSocket升级配置
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSConnection 封装WebSocket连接，提供线程安全的写操作
type WSConnection struct {
	Conn *websocket.Conn
	mu   sync.Mutex
}

// NewWSConnection 创建一个新的WSConnection实例
func NewWSConnection(conn *websocket.Conn) *WSConnection {
	return &WSConnection{
		Conn: conn,
	}
}

// WriteMessage 线程安全地写入消息
func (c *WSConnection) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(messageType, data)
}

// ReadLoop 循环读取消息，直到连接断开或回调返回错误
func (c *WSConnection) ReadLoop(handleMessage func(messageType int, data []byte) error) {
	defer func() {
		c.Conn.Close()
	}()

	for {
		messageType, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			}
			break
		}

		// 将消息交给业务层处理
		if err := handleMessage(messageType, message); err != nil {
			log.Printf("Error handling message: %v", err)
			break
		}
	}
}
