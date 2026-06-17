package services

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSUpgrader 标准的WebSocket升级配置
var WSUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const pongWait = 60 * time.Second

// WSConnection 封装WebSocket连接，提供线程安全的写操作
type WSConnection struct {
	Conn *websocket.Conn
	mu   sync.Mutex
}

// NewWSConnection 创建一个新的Connection实例
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

	// 设置 Pong 处理器：客户端回复 Pong 后刷新读超时
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		// 设置读超时（比 ping 周期长，防止连接僵死）
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))

		messageType, message, err := c.Conn.ReadMessage()
		if err != nil {
			log.Printf("WS ReadMessage error: %v", err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected close: %v", err)
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
