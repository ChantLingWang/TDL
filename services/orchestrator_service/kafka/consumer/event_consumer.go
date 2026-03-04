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
	GetSagaRepo() saga.SagaRepository
	GetInstanceID() string
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
		SagaRepo:      h.orchestrator.GetSagaRepo(),
		InstanceID:    h.orchestrator.GetInstanceID(),
	}

	var processErr error
	switch event.CommonParams.EventType {
	case saga.EventTypeSagaStart:
		processErr = handlers.HandleSagaStartEvent(sagaCtx)
	case saga.EventTypeStepSuccess:
		processErr = handlers.HandleStepSuccessEvent(sagaCtx)
	case saga.EventTypeStepFailed:
		processErr = handlers.HandleStepFailureEvent(sagaCtx)
	case saga.EventTypeStepRecoverySuccess:
		processErr = handlers.HandleStepRecoverySuccessEvent(sagaCtx)
	case saga.EventTypeStepRecoveryFail:
		processErr = handlers.HandleStepRecoveryFailureEvent(sagaCtx)
	default:
		// 忽略未知事件
	}

	if processErr != nil {
		log.Printf("❌ Error processing event %s (ID: %s): %v",
			event.CommonParams.EventType, event.CommonParams.EventID, processErr)
		// 直接返回错误，让 SDK 层进行重试
		return processErr
	}

	return nil
}
