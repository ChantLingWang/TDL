package kafka

import (
	"context"

	sdk_kafka "infrastructure_sdk/kafka"

	"github.com/google/uuid"
)

// KafkaProducer 包装 SDK 生产者，适配本地接口
type KafkaProducer struct {
	sdkProducer  *sdk_kafka.KafkaProducer
	defaultTopic string
}

// NewKafkaProducer 创建新的Kafka生产者
func NewKafkaProducer(connection *sdk_kafka.KafkaConnection, defaultTopic string) *KafkaProducer {
	return &KafkaProducer{
		sdkProducer:  sdk_kafka.NewKafkaProducer(connection),
		defaultTopic: defaultTopic,
	}
}

// SendEvent 通用发送方法
// key: 可选的分区键（如 UserID），传空字符串则轮询分区
func (kp *KafkaProducer) SendEvent(ctx context.Context, eventType string, key string, data interface{}) error {
	// 构造业务事件
	eventID := uuid.New().String()
	event, err := sdk_kafka.NewBusinessEvent(eventType, eventType, eventID, data)
	if err != nil {
		return err
	}

	// 传递配置的默认 topic
	return kp.sdkProducer.SendBusinessEvent(ctx, kp.defaultTopic, event, key)
}
