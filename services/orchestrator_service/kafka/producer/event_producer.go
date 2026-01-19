package producer

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"orchestrator_service/kafka"

	kafkago "github.com/segmentio/kafka-go"
)

// KafkaProducer Kafka事件生产者
type KafkaProducer struct {
	connection *kafka.KafkaConnection
}

// NewKafkaProducer 创建新的Kafka生产者
func NewKafkaProducer(connection *kafka.KafkaConnection) *KafkaProducer {
	return &KafkaProducer{
		connection: connection,
	}
}

// ========== 通用事件封装结构 ==========

// EventWrapper 通用事件包装结构
type EventWrapper struct {
	EventType string          `json:"event_type"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewEventWrapper 创建事件包装
func NewEventWrapper(eventType string, data interface{}) (*EventWrapper, error) {
	// 如果没有数据，直接返回包装
	if data == nil {
		return &EventWrapper{
			EventType: eventType,
			Data:      nil,
			Timestamp: time.Now(),
		}, nil
	}

	// 序列化数据
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &EventWrapper{
		EventType: eventType,
		Data:      dataBytes,
		Timestamp: time.Now(),
	}, nil
}

// ToBytes 转换为JSON字节
func (ew *EventWrapper) ToBytes() ([]byte, error) {
	return json.Marshal(ew)
}

// ========== 通用发送方法 ==========

// SendEvent 通用发送方法
func (kp *KafkaProducer) SendEvent(ctx context.Context, topic string, eventType string, sagaID string, data interface{}) error {
	// 创建事件包装
	eventWrapper, err := NewEventWrapper(eventType, data)
	if err != nil {
		log.Printf("Failed to create event wrapper: %v", err)
		return err
	}

	// 转换为JSON
	eventBytes, err := eventWrapper.ToBytes()
	if err != nil {
		log.Printf("Failed to serialize event: %v", err)
		return err
	}

	// 发送消息
	message := kafkago.Message{
		Topic: topic, // 动态指定Topic
		Value: eventBytes,
		Key:   []byte(sagaID), // 使用SagaID作为key，确保同一Saga的消息顺序
	}

	err = kp.connection.Writer.WriteMessages(ctx, message)
	if err != nil {
		log.Printf("Failed to send Kafka message (Topic: %s): %v", topic, err)
		return err
	}

	return nil
}
