package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// BaseConsumer 通用消费者框架
type BaseConsumer struct {
	connection *KafkaConnection
}

// NewBaseConsumer 创建通用消费者
func NewBaseConsumer(connection *KafkaConnection) *BaseConsumer {
	return &BaseConsumer{
		connection: connection,
	}
}

// MessageContext 包含消息元数据和提交操作
type MessageContext struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Timestamp time.Time
	commit    func() error
}

// Commit 提交当前消息的 Offset
func (mc *MessageContext) Commit() error {
	if mc.commit != nil {
		return mc.commit()
	}
	return nil
}

// fetch 从 Kafka 拉取一条业务事件
func (bc *BaseConsumer) fetch(ctx context.Context) (*BusinessEvent, *MessageContext, error) {
	reader := bc.connection.Reader

	// 1. Fetch Message
	msg, err := reader.FetchMessage(ctx)
	if err != nil {
		return nil, nil, err
	}

	// 2. Wrap Context
	msgCtx := &MessageContext{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Key:       msg.Key,
		Timestamp: msg.Time,
		commit: func() error {
			return reader.CommitMessages(context.Background(), msg)
		},
	}

	// 3. Unmarshal
	event := new(BusinessEvent)
	if err := json.Unmarshal(msg.Value, event); err != nil {
		// 反序列化失败，返回错误，但同时也返回 msgCtx
		return nil, msgCtx, fmt.Errorf("unmarshal failed: %w", err)
	}

	return event, msgCtx, nil
}

// HandlerFunc 业务处理函数定义
type HandlerFunc func(ctx context.Context, event *BusinessEvent) error

// Start 启动标准消费循环 (阻塞方法)
// 封装了 Fetch -> Handle -> Commit 的标准流程，确保 At-Least-Once 语义
func (bc *BaseConsumer) Start(ctx context.Context, handler HandlerFunc) error {
	log.Printf("Kafka consumer started for topic: %s", bc.connection.Reader.Config().Topic)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// 1. 主动拉取消息
			event, msgCtx, err := bc.fetch(ctx)
			if err != nil {
				// 检查是否是上下文取消
				if ctx.Err() != nil {
					return ctx.Err()
				}
				// 记录错误并进行退避，防止日志刷屏
				log.Printf("Kafka fetch error: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			// 2. 调用业务回调
			if err := handler(ctx, event); err != nil {
				// 业务处理失败
				log.Printf("Event handling failed (ID: %s, Type: %s): %v",
					event.CommonParams.EventID,
					event.CommonParams.EventType,
					err)

				time.Sleep(100 * time.Millisecond) // 简单限流
			} else {
				// 3. 业务处理成功，提交 Offset
				if err := msgCtx.Commit(); err != nil {
					log.Printf("Failed to commit offset for event %s: %v", event.CommonParams.EventID, err)
				}
			}
		}
	}
}
