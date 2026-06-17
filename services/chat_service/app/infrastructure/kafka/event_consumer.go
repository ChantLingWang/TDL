package kafka

import (
	"context"
	"encoding/json"
	"log"
	"chat_service/app/infrastructure/kafka/services"

	sdk_kafka "infrastructure_sdk/kafka"
)

// UserEventHandler 用户服务事件处理器
// 实现 sdk_kafka.EventHandler 接口
type UserEventHandler struct {
	// 回调函数
	chatMessageHandler      func(ctx context.Context, data json.RawMessage) error
	broadcastMessageHandler func(ctx context.Context, data json.RawMessage) error
}

// NewUserEventHandler 创建用户事件处理器
func NewUserEventHandler() *UserEventHandler {
	return &UserEventHandler{}
}

// SetChatMessageHandler 设置聊天消息处理回调
func (h *UserEventHandler) SetChatMessageHandler(handler func(ctx context.Context, data json.RawMessage) error) {
	h.chatMessageHandler = handler
}

// SetBroadcastMessageHandler 设置广播消息处理回调
func (h *UserEventHandler) SetBroadcastMessageHandler(handler func(ctx context.Context, data json.RawMessage) error) {
	h.broadcastMessageHandler = handler
}

// HandleEvent 实现 SDK 处理接口
func (h *UserEventHandler) HandleEvent(ctx context.Context, event *sdk_kafka.BusinessEvent) error {
	// 注意：编排器的事件类型存储在 CommonParams.EventType 中
	switch event.CommonParams.EventType {
// 处理聊天消息事件
	case EventUserChatMessage:
		if h.chatMessageHandler != nil {
			return h.chatMessageHandler(ctx, event.Data)
		}
		log.Printf("未设置聊天消息处理器，忽略事件")
		return nil

	// 处理广播消息事件
	case EventUserBroadcastMessage:
		if h.broadcastMessageHandler != nil {
			return h.broadcastMessageHandler(ctx, event.Data)
		}
		log.Printf("未设置广播消息处理器，忽略事件")
		return nil

	// 处理群聊消息事件
	case EventGroupChatMessage:
		return services.HandleGroupChatMessageEvent(ctx, event.Data)

	// 处理私聊消息事件
	case EventPrivateChatMessage:
		return services.HandlePrivateChatMessageEvent(ctx, event.Data)

	default:
		log.Printf("忽略未知事件类型: %s", event.CommonParams.EventType)
		return nil
	}
}
