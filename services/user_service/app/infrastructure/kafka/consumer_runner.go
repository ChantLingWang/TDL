package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"user_service/app/core"

	sdk_kafka "infrastructure_sdk/kafka"

	"github.com/google/uuid"
)

// ConsumerRunner 负责管理 Kafka 消费者的生命周期
type ConsumerRunner struct {
	config           core.KafkaConfig
	chatHandler      func(context.Context, json.RawMessage) error
	broadcastHandler func(context.Context, json.RawMessage) error
}

// NewConsumerRunner 创建新的消费者运行器
func NewConsumerRunner(
	chatHandler func(context.Context, json.RawMessage) error,
	broadcastHandler func(context.Context, json.RawMessage) error,
) *ConsumerRunner {
	return &ConsumerRunner{
		config:           core.KafkaConfigInstance,
		chatHandler:      chatHandler,
		broadcastHandler: broadcastHandler,
	}
}

// Run 启动消费者，阻塞直到上下文取消
func (r *ConsumerRunner) Run(ctx context.Context) error {
	// 1. 创建 Kafka 连接
	// 显式生成 GroupID (UUID) 以支持多实例
	groupID := uuid.New().String()
	log.Printf("Initializing Kafka consumer with GroupID: %s", groupID)

	connection, err := sdk_kafka.NewKafkaConnection(r.config.Brokers, r.config.Topic, groupID)
	if err != nil {
		return fmt.Errorf("failed to create kafka connection: %w", err)
	}
	defer func() {
		log.Println("Closing Kafka consumer connection...")
		connection.Close()
	}()

	// 2. 创建事件处理器并注册回调
	handler := NewUserEventHandler()
	handler.SetChatMessageHandler(r.chatHandler)
	handler.SetBroadcastMessageHandler(r.broadcastHandler)

	// 3. 创建 SDK 消费者
	consumer := sdk_kafka.NewBaseConsumer(connection)

	// 4. 启动消费循环
	if err := consumer.Start(ctx, handler.HandleEvent); err != nil {
		// 如果是上下文取消导致的错误，通常不视为异常
		if ctx.Err() != nil {
			log.Println("Kafka consumer stopped due to context cancellation")
			return nil
		}
		return fmt.Errorf("kafka consumer error: %w", err)
	}

	return nil
}
