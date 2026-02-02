package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

// KafkaProducer 通用 Kafka 生产者
type KafkaProducer struct {
	connection *KafkaConnection
}

// NewKafkaProducer 创建新的 Kafka 生产者
func NewKafkaProducer(connection *KafkaConnection) *KafkaProducer {
	return &KafkaProducer{
		connection: connection,
	}
}

// SendBusinessEvent 发送标准业务事件 (BusinessEvent 格式)
// topic: 目标 Topic (必填)
// key: 消息 Key，通常是 SagaID 或 UserID
func (kp *KafkaProducer) SendBusinessEvent(ctx context.Context, topic string, event *BusinessEvent, key string) error {
	msgBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal business event failed: %w", err)
	}

	return kp.writeMessage(ctx, topic, key, msgBytes)
}

// writeMessage 底层发送逻辑
func (kp *KafkaProducer) writeMessage(ctx context.Context, topic string, key string, value []byte) error {
	if topic == "" {
		return fmt.Errorf("kafka producer: topic is required")
	}

	msg := kafkago.Message{
		Value: value,
		Time:  time.Now(),
		Topic: topic,
	}

	if key != "" {
		msg.Key = []byte(key)
	}

	// 设置超时
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := kp.connection.Writer.WriteMessages(writeCtx, msg); err != nil {
		log.Printf("Failed to send Kafka message (Topic: %s): %v", topic, err)
		return err
	}

	return nil
}
