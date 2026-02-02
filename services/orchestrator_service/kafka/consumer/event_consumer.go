package consumer

import (
	"context"
	"log"
	"sync"

	"orchestrator_service/kafka/handlers"
	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"

	sdk_kafka "infrastructure_sdk/kafka"
)

// SagaOrchestratorInterface Saga编排器接口
type SagaOrchestratorInterface interface {
	GetKafkaProducer() *producer.KafkaProducer
	GetSagas() *map[string]*saga.Saga
	GetSagasMutex() *sync.RWMutex
}

// SagaEventHandler Saga事件处理器
type SagaEventHandler struct {
	orchestrator SagaOrchestratorInterface
}

// NewSagaEventHandler 创建Saga事件处理器
func NewSagaEventHandler(orchestrator SagaOrchestratorInterface) *SagaEventHandler {
	return &SagaEventHandler{
		orchestrator: orchestrator,
	}
}

// 负责分发 Saga 相关的业务事件
func (h *SagaEventHandler) HandleEvent(ctx context.Context, event *sdk_kafka.BusinessEvent) error {
	// 组装 SagaContext
	sagaCtx := &handlers.SagaEventHandlerContext{
		Ctx:           ctx,
		EventData:     event.Data,
		BusinessEvent: event,
		KafkaProducer: h.orchestrator.GetKafkaProducer(),
		Sagas:         *h.orchestrator.GetSagas(),
		SagasMutex:    h.orchestrator.GetSagasMutex(),
	}

	var processErr error
	switch event.CommonParams.EventType {
	case saga.EventTypeSagaStart:
		processErr = handlers.HandleSagaStartEvent(sagaCtx)
	case saga.EventTypeStepSuccess:
		processErr = handlers.HandleStepSuccessEvent(sagaCtx)
	case saga.EventTypeStepFailed:
		processErr = handlers.HandleStepFailureEvent(sagaCtx)
	default:
		// 忽略未知事件
	}

	if processErr != nil {
		log.Printf("❌ Error processing event %s (ID: %s): %v",
			event.CommonParams.EventType, event.CommonParams.EventID, processErr)
		// 注意：根据之前的逻辑，这里返回 nil 表示“已处理（即使失败）”，SDK 会提交 Offset。
		// 如果需要重试（At-Least-Once），应该返回 error。
		return nil
	}

	return nil
}

// Start 启动 Saga 事件消费者
func Start(ctx context.Context, consumer *sdk_kafka.BaseConsumer, orchestrator SagaOrchestratorInterface) error {
	log.Println("Starting Saga Orchestrator Consumer...")
	handler := NewSagaEventHandler(orchestrator)
	return consumer.Start(ctx, handler.HandleEvent)
}
