package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
	"user_service/app/infrastructure/kafka/model"

	"github.com/segmentio/kafka-go"
)

// KafkaProducer Kafka事件生产者
type KafkaProducer struct {
	connection *KafkaConnection
}

// NewKafkaProducer 创建新的Kafka生产者
func NewKafkaProducer(connection *KafkaConnection) *KafkaProducer {
	return &KafkaProducer{
		connection: connection,
	}
}

// ========== 通用事件封装结构 ==========

// NewEventWrapper 创建事件包装
func NewEventWrapper(eventType string, data interface{}) (*model.EventWrapper, error) {
	// 如果没有数据，直接返回包装
	if data == nil {
		return &model.EventWrapper{
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

	return &model.EventWrapper{
		EventType: eventType,
		Data:      dataBytes,
		Timestamp: time.Now(),
	}, nil
}

// ToBytes 转换为JSON字节
// 注意：这里我们定义一个辅助函数，或者直接在 model 中定义方法
// 由于 model 在另一个包，我们这里直接 marshaling
func ToBytes(ew *model.EventWrapper) ([]byte, error) {
	return json.Marshal(ew)
}

// ========== 通用发送方法 ==========

// SendEvent 通用发送方法
// key: 可选的分区键（如 UserID），传空字符串则轮询分区
func (kp *KafkaProducer) SendEvent(ctx context.Context, eventType string, key string, data interface{}) error {
	// 创建事件包装
	eventWrapper, err := NewEventWrapper(eventType, data)
	if err != nil {
		log.Printf("Failed to create event wrapper: %v", err)
		return err
	}

	// 转换为JSON
	eventBytes, err := ToBytes(eventWrapper)
	if err != nil {
		log.Printf("Failed to serialize event: %v", err)
		return err
	}

	// 构建消息
	msg := kafka.Message{
		Value: eventBytes,
		Time:  time.Now(),
	}

	// 如果提供了 Key，则设置
	if key != "" {
		msg.Key = []byte(key)
	}

	// 设置超时
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 发送消息
	if err := kp.connection.writer.WriteMessages(writeCtx, msg); err != nil {
		log.Printf("Failed to send Kafka message: %v", err)
		return fmt.Errorf("发送消息到 Kafka 失败: %w", err)
	}

	return nil
}
