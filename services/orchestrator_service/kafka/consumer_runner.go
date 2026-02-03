package kafka

import (
	"context"
	"fmt"
	"log"

	"orchestrator_service/kafka/consumer"
	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
	
	sdk_kafka "infrastructure_sdk/kafka"
	"sync"
)

// ConsumerRunner 负责管理 Kafka 消费者的生命周期
type ConsumerRunner struct {
	config       KafkaConfig
	orchestrator SagaOrchestratorInterface
}

// KafkaConfig 消费者运行器所需的配置
type KafkaConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

// SagaOrchestratorInterface 定义编排器接口 (避免循环依赖)
type SagaOrchestratorInterface interface {
	GetKafkaProducer() *producer.KafkaProducer
	GetSagas() *map[string]*saga.Saga
	GetSagasMutex() *sync.RWMutex
}

// NewConsumerRunner 创建新的消费者运行器
func NewConsumerRunner(
	config KafkaConfig,
	orchestrator SagaOrchestratorInterface,
) *ConsumerRunner {
	return &ConsumerRunner{
		config:       config,
		orchestrator: orchestrator,
	}
}

// Run 启动消费者，阻塞直到上下文取消
func (r *ConsumerRunner) Run(ctx context.Context) error {
	// 1. 创建 Kafka 连接
	connection, err := sdk_kafka.NewKafkaConnection(r.config.Brokers, r.config.Topic, r.config.GroupID)
	if err != nil {
		return fmt.Errorf("failed to create kafka connection: %w", err)
	}
	defer connection.Close()

	// 2. 创建 SDK 消费者
	baseConsumer := sdk_kafka.NewBaseConsumer(connection)

	// 3. 创建业务处理器 (使用之前已经存在的 SagaEventHandler)
	handler := consumer.NewSagaEventHandler(r.orchestrator)

	log.Printf("Starting Saga Orchestrator Consumer on topic: %s", r.config.Topic)

	// 4. 启动消费循环 (使用 SDK 的标准 Start 方法)
	if err := baseConsumer.Start(ctx, handler.HandleEvent); err != nil {
		if ctx.Err() != nil {
			log.Println("Kafka consumer stopped due to context cancellation")
			return nil
		}
		return fmt.Errorf("kafka consumer error: %w", err)
	}

	return nil
}
