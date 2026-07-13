package kafka

import (
	"context"
	"fmt"
	"log"

	config "chat_service/app/config"

	sdk_kafka "infrastructure_sdk/kafka"
)

// ConsumerRunner 负责管理 Kafka 消费者的生命周期（proto 版）
type ConsumerRunner struct {
	config config.KafkaConfig
}

// NewConsumerRunner 创建新的消费者运行器
func NewConsumerRunner() *ConsumerRunner {
	return &ConsumerRunner{
		config: config.KafkaConfigInstance,
	}
}

// Run 启动 proto 版消费循环，阻塞直到上下文取消
func (r *ConsumerRunner) Run(ctx context.Context) error {
	groupID := r.config.GroupID
	log.Printf("Initializing Kafka proto consumer with GroupID: %s", groupID)

	connection, err := sdk_kafka.NewKafkaConnection(r.config.Brokers, r.config.Topic, groupID)
	if err != nil {
		return fmt.Errorf("failed to create kafka connection: %w", err)
	}
	defer func() {
		log.Println("Closing Kafka consumer connection...")
		connection.Close()
	}()

	consumer := sdk_kafka.NewBaseConsumer(connection)

	// 注册 proto handler：根据 envelope.EventType 分发
	if err := consumer.StartProto(ctx, HandleProtoEnvelope); err != nil {
		if ctx.Err() != nil {
			log.Println("Kafka consumer stopped due to context cancellation")
			return nil
		}
		return fmt.Errorf("kafka consumer error: %w", err)
	}

	return nil
}
