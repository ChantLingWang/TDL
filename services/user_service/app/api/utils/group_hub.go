package utils

import (
	"log"
	"sync"
)

// Hub 维护着一组活跃的客户端连接
// 并负责消息的广播
type Hub struct {
	// 注册客户端的通道
	register chan *Client

	// 注销客户端的通道
	unregister chan *Client

	// 从客户端发送来的消息广播到其他客户端
	broadcast chan []byte

	// 组群映射，key为组群ID，value为该组群内的客户端集合
	groupID map[string]map[*Client]bool

	// 所有注册的客户端（用于全局广播）
	clients map[*Client]bool

	// 保护groupID和clients映射的互斥锁
	mu sync.RWMutex
}

var hub *Hub

// NewHub 创建一个新的Hub实例
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		groupID:    make(map[string]map[*Client]bool),
		clients:    make(map[*Client]bool),
	}
}

// GetHub 获取全局Hub实例
func GetHub() *Hub {
	return hub
}

// Run 启动Hub，处理各种通道消息
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)
		case client := <-h.unregister:
			h.handleUnregister(client)
		case message := <-h.broadcast:
			h.handleBroadcast(message)
		}
	}
}

// handleRegister 处理客户端注册
func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 将客户端添加到全局客户端集合中
	h.clients[client] = true

	// 如果客户端指定了组群ID，则将其添加到对应的组群中
		if client.GroupID != "" {
			// 如果组群存在，则将客户端添加到组群中
			if group, exists := h.groupID[client.GroupID]; exists {
				group[client] = true
				log.Printf("Client registered to group %s", client.GroupID)
			} else {
				// 如果组群不存在，记录警告日志
				log.Printf("Warning: Group %s does not exist, client not added to group", client.GroupID)
			}
		} else {
			log.Printf("Client registered without group")
		}
}

// handleUnregister 处理客户端注销
func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 从全局客户端集合中移除
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		// 不再在这里关闭client.Send通道，因为这会在readPump中处理

		// 如果客户端在某个组群中，则从该组群中移除
		if client.GroupID != "" {
			if group, groupExists := h.groupID[client.GroupID]; groupExists {
				if _, clientExists := group[client]; clientExists {
					delete(group, client)
					// 如果组群为空，则删除组群
					if len(group) == 0 {
						delete(h.groupID, client.GroupID)
					}
				}
			}
		}
		log.Printf("Client unregistered from group %s", client.GroupID)
	}
}

// handleBroadcast 处理消息广播
func (h *Hub) handleBroadcast(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 广播给所有客户端
	for client := range h.clients {
		select {
		case client.Send <- message:
		default:
			// 如果发送通道已满，则关闭连接
			close(client.Send)
			delete(h.clients, client)
			if client.GroupID != "" {
				if group, exists := h.groupID[client.GroupID]; exists {
					delete(group, client)
					if len(group) == 0 {
						delete(h.groupID, client.GroupID)
					}
				}
			}
		}
	}
}

// BroadcastToGroup 向指定房间内的所有客户端广播消息
func (h *Hub) BroadcastToGroup(groupID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 如果房间存在，则向房间内所有客户端发送消息
	if group, exists := h.groupID[groupID]; exists {
		for client := range group {
			select {
			case client.Send <- message:
			default:
				// 如果发送通道已满，则关闭连接
				close(client.Send)
				delete(h.clients, client)
				delete(group, client)
				if len(group) == 0 {
					delete(h.groupID, groupID)
				}
			}
		}
	}
}

// BroadcastToClient 向指定客户端发送消息
func (h *Hub) BroadcastToClient(client *Client, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 检查客户端是否仍然连接
	if _, exists := h.clients[client]; exists {
		select {
		case client.Send <- message:
		default:
			// 如果发送通道已满，则关闭连接
			close(client.Send)
			delete(h.clients, client)
			if client.GroupID != "" {
				if group, groupExists := h.groupID[client.GroupID]; groupExists {
					delete(group, client)
					if len(group) == 0 {
						delete(h.groupID, client.GroupID)
					}
				}
			}
		}
	}
}

// GetGroupClientsCount 获取指定房间内的客户端数量
func (h *Hub) GetGroupClientsCount(groupID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if group, exists := h.groupID[groupID]; exists {
		return len(group)
	}
	return 0
}

// GetGroups 获取所有房间列表
func (h *Hub) GetGroups() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	groups := make([]string, 0, len(h.groupID))
	for groupID := range h.groupID {
		groups = append(groups, groupID)
	}
	return groups
}

// GetAllClientsCount 获取所有客户端的数量
func (h *Hub) GetAllClientsCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	return len(h.clients)
}