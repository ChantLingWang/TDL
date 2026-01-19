package orchestrator

import (
	"context"
	"log"
	"sync"
	"time"

	"orchestrator_service/kafka/handlers"
	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
)

// SagaOrchestratorWithKafka 带Kafka的Saga编排器
type SagaOrchestratorWithKafka struct {
	// Saga管理
	sagas      map[string]*saga.Saga
	sagasMutex sync.RWMutex

	// 执行上下文
	ctx    context.Context
	cancel context.CancelFunc

	// Kafka生产者
	kafkaProducer *producer.KafkaProducer
}

// NewSagaOrchestratorWithKafka 创建带Kafka的Saga编排器
func NewSagaOrchestratorWithKafka(kafkaProducer *producer.KafkaProducer) *SagaOrchestratorWithKafka {
	ctx, cancel := context.WithCancel(context.Background())

	orchestrator := &SagaOrchestratorWithKafka{
		sagas:         make(map[string]*saga.Saga),
		ctx:           ctx,
		cancel:        cancel,
		kafkaProducer: kafkaProducer,
	}

	return orchestrator
}

// Shutdown 优雅关闭编排器
func (o *SagaOrchestratorWithKafka) Shutdown() {
	if o.cancel != nil {
		o.cancel()
	}
}

// GetKafkaProducer 获取Kafka生产者
func (o *SagaOrchestratorWithKafka) GetKafkaProducer() *producer.KafkaProducer {
	return o.kafkaProducer
}

// GetSagas 获取Saga映射
func (o *SagaOrchestratorWithKafka) GetSagas() *map[string]*saga.Saga {
	return &o.sagas
}

// GetSagasMutex 获取Saga互斥锁
func (o *SagaOrchestratorWithKafka) GetSagasMutex() *sync.RWMutex {
	return &o.sagasMutex
}

// GetContext 获取编排器的上下文
func (o *SagaOrchestratorWithKafka) GetContext() context.Context {
	return o.ctx
}

// CheckTimeouts 检查超时的Saga
func (o *SagaOrchestratorWithKafka) CheckTimeouts(timeoutThreshold time.Duration) {
	o.sagasMutex.RLock()
	// 创建快照以避免长时间持有锁
	sagasSnapshot := make([]*saga.Saga, 0, len(o.sagas))
	for _, s := range o.sagas {
		sagasSnapshot = append(sagasSnapshot, s)
	}
	o.sagasMutex.RUnlock()

	now := time.Now()
	for _, s := range sagasSnapshot {
		s.Mu.Lock()
		// 只有处于Running状态的Saga才需要检查超时
		if s.Status == saga.StatusRunning {
			// 如果最后更新时间超过阈值
			if now.Sub(s.UpdatedAt) > timeoutThreshold {
				log.Printf("⚠️ Saga %s timed out (last updated: %v). Triggering compensation...", s.ID, s.UpdatedAt)

				// 标记为失败
				s.Status = saga.StatusFailed
				s.UpdatedAt = now

				// 获取当前需要回滚的步骤索引（对于并行执行，通常全量回滚；对于串行，回滚已执行的）
				// 这里为了简化，我们传入 -1 或者当前步骤索引，由 compensateExecutedSteps 内部逻辑处理
				failedStepIndex := -1
				if s.CurrentStep >= 0 {
					failedStepIndex = s.CurrentStep
				}

				// 释放锁后再调用补偿逻辑，防止死锁
				s.Mu.Unlock()

				// 触发补偿
				handlers.TriggerSagaCompensation(o.ctx, s, failedStepIndex, o.kafkaProducer)
				continue
			}
		}
		s.Mu.Unlock()
	}
}
