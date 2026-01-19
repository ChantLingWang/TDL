package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"
	"user_service/app/infrastructure/kafka/model"
	"user_service/app/infrastructure/kafka/services"
)

// KafkaConsumer Kafka交互层，负责使用连接与Kafka交互
type KafkaConsumer struct {
	connection *KafkaConnection
}

// NewKafkaConsumer 创建新的Kafka消费者
func NewKafkaConsumer(connection *KafkaConnection) *KafkaConsumer {
	return &KafkaConsumer{
		connection: connection,
	}
}

// ========== 事件消费者架构 ==========

// BaseEventHandler 基础事件处理器，封装通用消费逻辑
type BaseEventHandler struct {
	connection *KafkaConnection

	// 回调函数
	chatMessageHandler      func(ctx context.Context, data json.RawMessage) error
	broadcastMessageHandler func(ctx context.Context, data json.RawMessage) error
}

// NewBaseEventHandler 创建基础事件处理器
func NewBaseEventHandler(connection *KafkaConnection) *BaseEventHandler {
	return &BaseEventHandler{
		connection: connection,
	}
}

// SetChatMessageHandler 设置聊天消息处理回调
func (bh *BaseEventHandler) SetChatMessageHandler(handler func(ctx context.Context, data json.RawMessage) error) {
	bh.chatMessageHandler = handler
}

// SetBroadcastMessageHandler 设置广播消息处理回调
func (bh *BaseEventHandler) SetBroadcastMessageHandler(handler func(ctx context.Context, data json.RawMessage) error) {
	bh.broadcastMessageHandler = handler
}

// parseOuterStructure 解析外层结构（通用逻辑）
func (bh *BaseEventHandler) parseOuterStructure(msg []byte) (*model.BusinessEvent, error) {
	var businessEvent model.BusinessEvent
	if err := json.Unmarshal(msg, &businessEvent); err != nil {
		return nil, err
	}
	return &businessEvent, nil
}

// ConsumeEvents 消费事件的模板方法（通用逻辑）
func (bh *BaseEventHandler) ConsumeEvents(ctx context.Context) error {
	reader := bh.connection.reader

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// 设置读取超时
			readCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			msg, err := reader.ReadMessage(readCtx)
			cancel()

			if err != nil {
				// 忽略超时错误，不打印日志
				// kafka-go 可能返回包装后的错误，所以检查错误信息
				if err == context.DeadlineExceeded || err.Error() == "fetching message: context deadline exceeded" {
					continue
				}
				log.Printf("读取Kafka消息失败: %v", err)
				continue
			}

			// 解析外层结构
			businessEvent, err := bh.parseOuterStructure(msg.Value)
			if err != nil {
				log.Printf("解析业务事件失败: %v", err)
				continue
			}

			// 3. 调用分发器处理事件
			if err := bh.dispatchEvent(ctx, businessEvent); err != nil {
				log.Printf("事件处理失败: %v", err)
				continue
			}
		}
	}
}

// dispatchEvent 事件分发器（中转函数）
func (bh *BaseEventHandler) dispatchEvent(ctx context.Context, event *model.BusinessEvent) error {
	// 注意：编排器的事件类型存储在 CommonParams.EventType 中
	switch event.CommonParams.EventType {
	// 处理用户注册事件
	case EventUserRegistered:
		return services.HandleUserRegisteredEvent(ctx, event.Data)

	// 处理踢人事件
	case EventUserKick:
		// TODO: 需要在 services 包中实现 HandleUserKickEvent
		// return services.HandleUserKickEvent(ctx, event.Data)
		return nil

	// 处理聊天消息事件
	case EventUserChatMessage:
		if bh.chatMessageHandler != nil {
			return bh.chatMessageHandler(ctx, event.Data)
		}
		log.Printf("未设置聊天消息处理器，忽略事件")
		return nil

	// 处理广播消息事件
	case EventUserBroadcastMessage:
		if bh.broadcastMessageHandler != nil {
			return bh.broadcastMessageHandler(ctx, event.Data)
		}
		log.Printf("未设置广播消息处理器，忽略事件")
		return nil

	default:
		log.Printf("忽略未知事件类型: %s", event.CommonParams.EventType)
		return nil
	}
}
