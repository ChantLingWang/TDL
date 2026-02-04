package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// HandlerFunc 业务处理函数定义
type HandlerFunc func(ctx context.Context, event *BusinessEvent) error

// BaseConsumer 通用消费者框架
type BaseConsumer struct {
	connection  *KafkaConnection
	dlqProducer *KafkaProducer // 可选：死信队列生产者
	dlqTopic    string         // 可选：死信队列 Topic
}

// NewBaseConsumer 创建通用消费者
func NewBaseConsumer(connection *KafkaConnection) *BaseConsumer {
	return &BaseConsumer{
		connection: connection,
	}
}

// SetDLQ 配置死信队列
func (bc *BaseConsumer) SetDLQ(producer *KafkaProducer, topic string) {
	bc.dlqProducer = producer
	bc.dlqTopic = topic
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

// performBackoff 执行指数退避逻辑
func performBackoff(ctx context.Context, attempt int, baseDelay time.Duration, eventID string) bool {
	if attempt <= 0 {
		return false
	}

	// 指数退避: 1s, 2s, 4s
	backoff := baseDelay * time.Duration(1<<uint(attempt-1))
	log.Printf("Retrying event %s (attempt %d) after %v...", eventID, attempt, backoff)

	select {
	case <-ctx.Done():
		return true // 上下文取消
	case <-time.After(backoff):
		return false // 继续重试
	}
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

// Start 启动标准消费循环 (阻塞方法)
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

			// 2. 调用业务回调 (带重试机制)
			bc.executeWithRetry(ctx, handler, event, msgCtx.Topic)

			// 3. 提交 Offset
			// 只有当 handler 成功，或者重试次数耗尽我们决定跳过时，才提交
			if err := msgCtx.Commit(); err != nil {
				log.Printf("Kafka commit error: %v", err)
			}
		}
	}
}

// executeWithRetry 执行业务回调并处理重试逻辑
func (bc *BaseConsumer) executeWithRetry(ctx context.Context, handler HandlerFunc, event *BusinessEvent, originalTopic string) {
	const (
		maxRetries = 3
		baseDelay  = 1 * time.Second
	)

	var handleErr error
	success := false

	for i := 0; i <= maxRetries; i++ {
		// 每次重试前，执行退避逻辑
		if shouldStop := performBackoff(ctx, i, baseDelay, event.CommonParams.EventID); shouldStop {
			return
		}

		handleErr = handler(ctx, event)
		if handleErr == nil {
			success = true
			break
		}
		log.Printf("Handler error for event %s: %v", event.CommonParams.EventID, handleErr)
	}

	if !success {
		log.Printf("❌ Failed to process event %s after %d attempts. Skipping to avoid blocking queue.", event.CommonParams.EventID, maxRetries)
		// 如果配置了 DLQ，投递到死信队列
		if bc.dlqProducer != nil && bc.dlqTopic != "" {
			if err := bc.sendToDLQ(ctx, originalTopic, event, handleErr); err != nil {
				log.Printf("Failed to send to DLQ: %v", err)
			}
		}
		// 注意：这里我们选择提交 Offset，即使处理失败。
	}
}

// sendToDLQ 发送消息到死信队列
func (bc *BaseConsumer) sendToDLQ(ctx context.Context, originalTopic string, event *BusinessEvent, reasonErr error) error {
	reason := "unknown"
	if reasonErr != nil {
		reason = reasonErr.Error()
	}

	dlqPayload := NewDLQPayload(originalTopic, event, reason)

	// 构造 DLQ 事件
	dlqEvent, err := NewBusinessEvent(DLQEventType, "DLQ Message", event.CommonParams.EventID, dlqPayload)
	if err != nil {
		return fmt.Errorf("failed to create DLQ event: %w", err)
	}

	// 发送
	return bc.dlqProducer.SendBusinessEvent(ctx, bc.dlqTopic, dlqEvent, event.CommonParams.EventID)
}
