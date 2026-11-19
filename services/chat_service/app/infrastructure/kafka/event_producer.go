package kafka

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
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
	globalProducer = p
	return p
}

// GetProducer 获取全局生产者实例
func GetProducer() *KafkaProducer {
	return globalProducer
}

// SendProtoEvent 用 proto 消息类型发送事件信封。
// eventType 如 "chant.chat.v1.MessageSent"
func (kp *KafkaProducer) SendProtoEvent(ctx context.Context, eventType, key string, msg proto.Message) error {
	envelope, err := sdk_kafka.NewEventEnvelope(eventType, "chat-service", msg)
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	// 使用 proto 消息自身的 message_id 作为事件 ID
	if mid, ok := msg.(interface{ GetMessageId() string }); ok && mid.GetMessageId() != "" {
		envelope.EventId = mid.GetMessageId()
	}
	envelope.Timestamp = time.Now().UnixMilli()
	return kp.sdkProducer.SendEnvelope(ctx, kp.defaultTopic, key, envelope)
}
