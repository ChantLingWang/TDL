package services

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

// UserInfo 用户信息结构
type UserInfo struct {
	UserID   string
	Username string
	Email    string
}

// WSClient 代表一个连接到 Hub 的 WebSocket 客户端
type WSClient struct {
	Hub  *WSHub
	Conn *WSConnection

	// Send 是用于向客户端发送消息的缓冲通道
	Send chan []byte

	// 客户端用户信息
	UserInfo *UserInfo
}

// WSHub 维护活跃客户端并处理广播
type WSHub struct {
	// 注册和注销通道
	register   chan *WSClient
	unregister chan *WSClient

	// 客户端索引
	// key: UserID, value: WSClient 指针
	clients map[string]*WSClient

	// 读写锁，保护 clients map
	mu sync.RWMutex
}


var (
	hubInstance *WSHub
	once        sync.Once
)


// NewWSClient 创建一个新的客户端实例
func NewWSClient(hub *WSHub, conn *WSConnection, userInfo *UserInfo) *WSClient {
	return &WSClient{
		Hub:      hub,
		Conn:     conn,
		Send:     make(chan []byte, 256), // 设置256的缓冲区，防止阻塞
		UserInfo: userInfo,
	}
}

// GetWSHub 获取单例 Hub 实例
func GetWSHub() *WSHub {
	once.Do(func() {
		hubInstance = &WSHub{
			register:   make(chan *WSClient),
			unregister: make(chan *WSClient),
			clients:    make(map[string]*WSClient),
		}
		go hubInstance.Run()
	})
	return hubInstance
}


// WritePump 监听 Send 通道并将消息写入 WebSocket 连接
func (c *WSClient) WritePump() {
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
				log.Printf("Client %s write error: %v", c.UserInfo.UserID, err)
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
func (h *WSHub) Run() {
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
func (h *WSHub) Register(client *WSClient) {
	h.register <- client
}

// Unregister 注销客户端（对外接口）
func (h *WSHub) Unregister(client *WSClient) {
	h.unregister <- client
}

// BroadcastToUser 向指定用户发送消息
// 如果用户在线，直接发送；如果用户不在线，返回 false
func (h *WSHub) BroadcastToUser(userID string, message []byte) bool {
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

// GetOnlineCount 获取当前在线用户数
func (h *WSHub) GetOnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// IsUserOnline 检查用户是否在线
func (h *WSHub) IsUserOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}

// KickUser 强制用户下线
// 返回值: true 表示成功踢人，false 表示用户本来就不在线
func (h *WSHub) KickUser(userID string) bool {
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
func (h *WSHub) registerClient(client *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 检查是否已有旧连接
	if oldClient, ok := h.clients[client.UserInfo.UserID]; ok {
		// 踢掉旧连接（单端登录策略）
		// 关闭旧连接的 Send 通道会触发其 WritePump 退出并关闭 TCP 连接
		close(oldClient.Send)
		delete(h.clients, client.UserInfo.UserID)
	}

	h.clients[client.UserInfo.UserID] = client
}

// unregisterClient 内部注销逻辑
func (h *WSHub) unregisterClient(client *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 只有当 map 里的 client 和传入的 client 是同一个实例时才删除
	// 防止误删了刚刚注册的新连接（如果发生了快速重连）
	if currentClient, ok := h.clients[client.UserInfo.UserID]; ok && currentClient == client {
		delete(h.clients, client.UserInfo.UserID)
		close(client.Send)
	}
}
