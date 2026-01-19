package deadlinequeue

import (
	"context"
	"log"
	"time"

	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
)

// DLQMessage 死信队列消息结构
type DLQMessage struct {
	SagaID       string      `json:"saga_id"`
	StepIndex    int         `json:"step_index"`
	ServiceName  string      `json:"service_name"`
	Reason       string      `json:"reason"`
	OriginalData any         `json:"original_data"`
	FailedAt     time.Time   `json:"failed_at"`
}

// SendToDLQ 发送消息到死信队列
func SendToDLQ(ctx context.Context, kafkaProducer *producer.KafkaProducer, sagaID string, stepIndex int, serviceName string, reason string, originalData interface{}) error {
	dlqMsg := DLQMessage{
		SagaID:       sagaID,
		StepIndex:    stepIndex,
		ServiceName:  serviceName,
		Reason:       reason,
		OriginalData: originalData,
		FailedAt:     time.Now(),
	}

	// 统一使用 saga-dlq Topic
	dlqTopic := saga.TopicSagaDLQ

	log.Printf("Sending message to DLQ (Topic: %s): SagaID=%s, Step=%d, Reason=%s", dlqTopic, sagaID, stepIndex, reason)

	// 复用通用的 SendEvent 方法发送
	return kafkaProducer.SendEvent(ctx, dlqTopic, saga.EventTypeDLQMessage, sagaID, dlqMsg)
}
