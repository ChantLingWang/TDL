package utils

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// 初始化Hub
func init() {
	hub = NewHub()
	go hub.Run()
}

// 心跳机制的常量定义
const (
	// 从对端读取消息的等待时间
	pongWait = 60 * time.Second
	// Ping 消息的发送周期，必须小于 pongWait
	pingPeriod = (pongWait * 9) / 10
	// 写入消息到对端的等待时间
	writeWait = 10 * time.Second
)

// 标准的WebSocket升级配置
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 生产环境中应加入严格的来源检查
	CheckOrigin: func(r *http.Request) bool {
		// 这里应该有更复杂的逻辑，比如检查 r.Header.Get("Origin") 是否在白名单内
		return true
	},
}

// Client 表示一个WebSocket客户端连接，以及其发送消息的缓冲通道
type Client struct {
	Conn   *websocket.Conn
	Send   chan []byte
	GroupID string
}


// WebSocketHandler 负责处理WebSocket升级请求和管理客户端连接的生命周期
func WebSocketHandler(c *gin.Context) {
	// 1. 将HTTP连接升级为WebSocket连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("webSocket upgrade failed:", err)
		return
	}

	// 2. 从URL参数中获取房间ID
	groupID := c.Param("group_id")
	if groupID == "" {
		groupID = "default" // 如果没有指定房间，则使用默认房间
	}
	log.Printf("Client connected to group: %s", groupID)

	// 3. 创建客户端实例
	client := &Client{
		Conn:   conn,
		Send:   make(chan []byte, 256),
		GroupID: groupID,
	}

	// 4. 将客户端注册到Hub中
	hub.register <- client
	log.Printf("Client registered to hub")

	// 5. 启动一个协程，负责从 Send 通道读取消息并写入 WebSocket 连接
	go client.writePump()

	// 6. 在当前主协程中运行 readPump，它会阻塞直到连接断开
	// 这样做的好处是，Handler 的生命周期与连接的生命周期完全绑定
	client.readPump()

	// 7. 当 readPump 返回后，意味着连接已断开，将客户端从Hub中注销
	hub.unregister <- client
	log.Println("Client connection closed, handler finished.")
}

// readPump 负责从 WebSocket 连接中循环读取消息
func (c *Client) readPump() {
	// 当读取循环（for循环）因任何原因（如连接断开、读取错误）return时，
	//这个 defer 语句会确保连接被关闭，并且 `Send` 通道也被关闭，
	// 从而通知 writePump 停止工作并退出。
	defer func() {
		c.Conn.Close()
		close(c.Send)
		log.Println("readPump exited and cleaned up resources.")
	}()

	// 设置心跳机制
	// 设置初始的读取超时时间
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	// 设置 Pong 消息的处理器，收到 Pong 后更新读取超时时间
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// 无限循环，持续读取消息
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			// IsUnexpectedCloseError 用于判断是否是“非预期”的关闭错误
			// 比如浏览器关闭、Tab页关闭等属于正常关闭，不需要作为错误日志打印
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			} else {
				log.Printf("WebSocket connection closed normally.")
			}
			break // 跳出循环，触发 defer 中的清理逻辑
		}

		// **业务逻辑处理点**
		// 在一个真实的聊天应用中，这里应该将 message 解析，
		// 然后分发到业务逻辑层处理，例如通过Hub广播给房间内的其他用户。
		log.Printf("Received message: %s from group: %s", message, c.GroupID)
		
		// 将消息发送到Hub进行广播
		hub.BroadcastToGroup(c.GroupID, message)
	}
}

// writePump 负责从 Send 通道中获取消息并将其写入 WebSocket 连接
func (c *Client) writePump() {
	// 创建一个定时器，用于定期发送 Ping 消息以保持连接活跃
	ticker := time.NewTicker(pingPeriod)

	// 连接的关闭由 readPump 统一管理。
	defer func() {
		ticker.Stop()
		log.Println("writePump exited.")
	}()

	for {
		select {
		case message, ok := <-c.Send:
			// 设置写入超时
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Send 通道被关闭，这是 readPump 发出的信号，表明连接已关闭
				// 发送一个关闭消息给对端，然后退出协程
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// `WriteMessage` 会为每条消息创建一个独立的帧。
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return // 写入失败，表明连接已断开，退出协程
			}

		case <-ticker.C:
			// 定时器触发，发送 Ping 消息
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			// WriteMessage 是并发安全的
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Ping failed: %v", err)
				return // Ping 失败，表明连接已断开，退出协程
			}
		}
	}
}

