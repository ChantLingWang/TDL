package producer

import (
	"context"
	sdk_kafka "infrastructure_sdk/kafka"
)

// KafkaProducer Kafka事件生产者
type KafkaProducer struct {
	sdkProducer *sdk_kafka.KafkaProducer
}

// NewKafkaProducer 创建新的Kafka生产者
func NewKafkaProducer(connection *sdk_kafka.KafkaConnection) *KafkaProducer {
	return &KafkaProducer{
		sdkProducer: sdk_kafka.NewKafkaProducer(connection),
	}
}

func (kp *KafkaProducer) SendEvent(ctx context.Context, topic string, eventType string, sagaID string, data interface{}) error {
	event, err := sdk_kafka.NewBusinessEvent(eventType, eventType, sagaID, data)
	if err != nil {
		return err
	}

	// 使用 SDK 发送标准业务事件
	return kp.sdkProducer.SendBusinessEvent(ctx, topic, event, sagaID)
}
