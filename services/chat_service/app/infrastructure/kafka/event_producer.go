package kafka

import (
	"context"

	sdk_kafka "infrastructure_sdk/kafka"
)

// KafkaProducer 包装 SDK 生产者，适配本地接口
type KafkaProducer struct {
	sdkProducer  *sdk_kafka.KafkaProducer
	defaultTopic string
}

// 全局生产者单例
var globalProducer *KafkaProducer

// NewKafkaProducer 创建新的Kafka生产者
func NewKafkaProducer(connection *sdk_kafka.KafkaConnection, defaultTopic string) *KafkaProducer {
	p := &KafkaProducer{
		sdkProducer:  sdk_kafka.NewKafkaProducer(connection),
		defaultTopic: defaultTopic,
	}
	// 保存为全局单例
	globalProducer = p
	return p
}

// GetProducer 获取全局生产者实例
func GetProducer() *KafkaProducer {
	return globalProducer
}

// SendEvent 通用发送方法
// messageID 用于标识消息，用于 Kafka 事件的 eventID
func (kp *KafkaProducer) SendEvent(ctx context.Context, eventType string, messageID string, key string, data interface{}) error {
	// 使用传入的 messageID 作为 eventID
	event, err := sdk_kafka.NewBusinessEvent(eventType, eventType, messageID, data)
	if err != nil {
		return err
	}

	// 传递配置的默认 topic
	return kp.sdkProducer.SendBusinessEvent(ctx, kp.defaultTopic, event, key)
}
