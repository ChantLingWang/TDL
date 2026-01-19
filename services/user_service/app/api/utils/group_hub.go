package utils

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Ping 消息发送周期，必须小于客户端的读取超时时间（PongWait）
	pingPeriod = 50 * time.Second
)

// Client 代表一个连接到 Hub 的 WebSocket 客户端
type Client struct {
	Hub  *Hub
	Conn *WSConnection

	// Send 是用于向客户端发送消息的缓冲通道
	Send chan []byte

	// 客户端标识
	UserID string
}

// Hub 维护活跃客户端并处理广播
type Hub struct {
	// 注册和注销通道
	register   chan *Client
	unregister chan *Client

	// 客户端索引
	// key: UserID, value: Client 指针
	clients map[string]*Client

	// 读写锁，保护 clients map
	mu sync.RWMutex
}


var (
	hubInstance *Hub
	once        sync.Once
)


// NewClient 创建一个新的客户端实例
func NewClient(hub *Hub, conn *WSConnection, userID string) *Client {
	return &Client{
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan []byte, 256), // 设置256的缓冲区，防止阻塞
		UserID: userID,
	}
}

// GetHub 获取单例 Hub 实例
func GetHub() *Hub {
	once.Do(func() {
		hubInstance = &Hub{
			register:   make(chan *Client),
			unregister: make(chan *Client),
			clients:    make(map[string]*Client),
		}
		go hubInstance.Run()
	})
	return hubInstance
}


// WritePump 监听 Send 通道并将消息写入 WebSocket 连接
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Conn.Close() // 关闭底层连接
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				// Hub 关闭了通道，发送 Close 帧
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("Client %s write error: %v", c.UserID, err)
				return
			}

		case <-ticker.C:
			// 定时发送 Ping 消息以保持连接活跃
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}


// Run 启动 Hub 的主循环，处理注册和注销请求
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		}
	}
}

// Register 注册客户端（对外接口）
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister 注销客户端（对外接口）
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// BroadcastToUser 向指定用户发送消息
// 如果用户在线，直接发送；如果用户不在线，返回 false
func (h *Hub) BroadcastToUser(userID string, message []byte) bool {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if ok {
		select {
		case client.Send <- message:
			return true
		default:
			// 通道已满，强制断开以保护 Hub
			h.Unregister(client) // 异步注销
			return false
		}
	}
	return false
}

// BroadcastToUsers 批量向用户发送消息（通常用于群发）
// userIDs: 目标用户ID列表
func (h *Hub) BroadcastToUsers(userIDs []string, message []byte) {
	for _, userID := range userIDs {
		h.BroadcastToUser(userID, message)
	}
}

// GetOnlineCount 获取当前在线用户数
func (h *Hub) GetOnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// IsUserOnline 检查用户是否在线
func (h *Hub) IsUserOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}

// KickUser 强制用户下线
// 返回值: true 表示成功踢人，false 表示用户本来就不在线
func (h *Hub) KickUser(userID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if client, ok := h.clients[userID]; ok {
		// 删除映射
		delete(h.clients, userID)
		// 关闭通道，这会触发 WritePump 退出并关闭底层连接
		close(client.Send)
		return true
	}
	return false
}

// registerClient 内部注册逻辑
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 检查是否已有旧连接
	if oldClient, ok := h.clients[client.UserID]; ok {
		// 踢掉旧连接（单端登录策略）
		// 关闭旧连接的 Send 通道会触发其 WritePump 退出并关闭 TCP 连接
		close(oldClient.Send)
		delete(h.clients, client.UserID)
	}

	h.clients[client.UserID] = client
}

// unregisterClient 内部注销逻辑
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 只有当 map 里的 client 和传入的 client 是同一个实例时才删除
	// 防止误删了刚刚注册的新连接（如果发生了快速重连）
	if currentClient, ok := h.clients[client.UserID]; ok && currentClient == client {
		delete(h.clients, client.UserID)
		close(client.Send)
	}
}
