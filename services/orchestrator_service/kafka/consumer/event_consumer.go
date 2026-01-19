package consumer

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"orchestrator_service/kafka"
	consumer_model "orchestrator_service/kafka/consumer/model"
	"orchestrator_service/kafka/handlers"
	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
)

// SagaOrchestratorInterface Saga编排器接口
type SagaOrchestratorInterface interface {
	GetKafkaProducer() *producer.KafkaProducer
	GetSagas() *map[string]*saga.Saga
	GetSagasMutex() *sync.RWMutex
}

// BaseEventHandler 基础事件处理器，封装通用消费逻辑
type BaseEventHandler struct {
	connection   *kafka.KafkaConnection
	orchestrator SagaOrchestratorInterface
}

// NewBaseEventHandler 创建基础事件处理器
func NewBaseEventHandler(connection *kafka.KafkaConnection, orchestrator SagaOrchestratorInterface) *BaseEventHandler {
	return &BaseEventHandler{
		connection:   connection,
		orchestrator: orchestrator,
	}
}

// parseOuterStructure 解析外层结构（通用逻辑）
func (bh *BaseEventHandler) parseOuterStructure(msg []byte) (*consumer_model.BusinessEvent, error) {
	var businessEvent consumer_model.BusinessEvent
	if err := json.Unmarshal(msg, &businessEvent); err != nil {
		return nil, err
	}
	return &businessEvent, nil
}

// ConsumeEvents 消费事件的模板方法
func (bh *BaseEventHandler) ConsumeEvents(ctx context.Context) error {
	reader := bh.connection.Reader

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// 设置读取超时
			readCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			msg, err := reader.ReadMessage(readCtx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded {
					continue // 超时继续循环
				}
				continue
			}

			// 解析外层结构
			businessEvent, err := bh.parseOuterStructure(msg.Value)
			if err != nil {
				continue
			}

			// 事务协调分发逻辑 - 编排器接收启动事件和服务反馈消息
			switch businessEvent.CommonParams.EventType {
			case saga.EventTypeSagaStart:
				if err := handlers.HandleSagaStartEvent(
					ctx,
					businessEvent,
					bh.orchestrator.GetKafkaProducer(),
					*bh.orchestrator.GetSagas(),
					bh.orchestrator.GetSagasMutex()); err != nil {
					log.Printf("❌ Failed to process saga start event: %v", err)
					continue
				}

			case saga.EventTypeStepSuccess:
				if err := handlers.HandleStepSuccessEvent(
					ctx,
					businessEvent.Data,
					bh.orchestrator.GetKafkaProducer(),
					*bh.orchestrator.GetSagas(),
					bh.orchestrator.GetSagasMutex()); err != nil {
					log.Printf("❌ Failed to process step success event: %v", err)
					continue
				}

			case saga.EventTypeStepFailed:
				if err := handlers.HandleStepFailureEvent(
					ctx,
					businessEvent.Data,
					bh.orchestrator.GetKafkaProducer(),
					*bh.orchestrator.GetSagas(),
					bh.orchestrator.GetSagasMutex()); err != nil {
					log.Printf("❌ Failed to process step failure event: %v", err)
					continue
				}

			case saga.EventTypeStepRecoveryFail:
				if err := handlers.HandleStepRecoveryFailureEvent(
					ctx,
					businessEvent.Data,
					bh.orchestrator.GetKafkaProducer(),
					*bh.orchestrator.GetSagas(),
					bh.orchestrator.GetSagasMutex()); err != nil {
					log.Printf("❌ Failed to process step compensation failure event: %v", err)
					continue
				}

			case saga.EventTypeStepRecoverySuccess:
				if err := handlers.HandleStepRecoverySuccessEvent(
					ctx,
					businessEvent.Data,
					bh.orchestrator.GetKafkaProducer(),
					*bh.orchestrator.GetSagas(),
					bh.orchestrator.GetSagasMutex()); err != nil {
					log.Printf("❌ Failed to process step compensation success event: %v", err)
					continue
				}

			default:
				log.Printf("⚠️ Received unhandled event type: %s", businessEvent.CommonParams.EventName)
			}
		}
	}
}
