package orchestrator

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"orchestrator_service/kafka/handlers"
	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"

	"github.com/google/uuid"
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

	// Saga持久化仓库
	sagaRepo saga.SagaRepository

	// 实例ID，用于分布式锁
	instanceID string
}

// NewSagaOrchestratorWithKafka 创建带Kafka的Saga编排器
func NewSagaOrchestratorWithKafka(kafkaProducer *producer.KafkaProducer, sagaRepo saga.SagaRepository) *SagaOrchestratorWithKafka {
	ctx, cancel := context.WithCancel(context.Background())

	orchestrator := &SagaOrchestratorWithKafka{
		sagas:         make(map[string]*saga.Saga),
		ctx:           ctx,
		cancel:        cancel,
		kafkaProducer: kafkaProducer,
		sagaRepo:      sagaRepo,
		instanceID:    uuid.New().String(),
	}

	return orchestrator
}

// GetSagaRepo 获取Saga仓库
func (o *SagaOrchestratorWithKafka) GetSagaRepo() saga.SagaRepository {
	return o.sagaRepo
}

// GetInstanceID 获取实例ID
func (o *SagaOrchestratorWithKafka) GetInstanceID() string {
	return o.instanceID
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

// saveWithOptimisticLockRetry 使用乐观锁保存Saga，自动处理版本冲突重试
func (o *SagaOrchestratorWithKafka) saveWithOptimisticLockRetry(
	sagaInstance *saga.Saga,
	maxRetries int,
	updateFn func(s *saga.Saga) bool,
) error {
	if o.sagaRepo == nil {
		return nil
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var currentSaga *saga.Saga
		if attempt == 0 {
			currentSaga = sagaInstance
		} else {
			var err error
			currentSaga, err = o.sagaRepo.Get(o.ctx, sagaInstance.ID)
			if err != nil {
				if errors.Is(err, saga.ErrSagaNotFound) {
					log.Printf("⚠️ Saga %s no longer exists, skipping save", sagaInstance.ID)
					return nil
				}
				return err
			}
			sagaInstance.Version = currentSaga.Version
		}

		if !updateFn(currentSaga) {
			return nil
		}

		err := o.sagaRepo.Save(o.ctx, currentSaga)
		if err == nil {
			sagaInstance.Version = currentSaga.Version
			return nil
		}

		if errors.Is(err, saga.ErrVersionConflict) {
			if attempt < maxRetries {
				log.Printf("⚠️ Saga %s version conflict (attempt %d/%d), retrying...",
					sagaInstance.ID, attempt+1, maxRetries+1)
				continue
			}
		}
		return err
	}
	return nil
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

				// 获取当前需要回滚的步骤索引
				failedStepIndex := -1
				if s.CurrentStep >= 0 {
					failedStepIndex = s.CurrentStep
				}

				// 在持有锁的情况下设置状态为 Compensating
				s.Status = saga.StatusCompensating
				s.UpdatedAt = now
				// 注意：不再手动增加 Version，Save 方法会自动处理

				// 释放锁后再调用补偿逻辑，防止死锁
				s.Mu.Unlock()

				// 持久化超时触发的补偿状态
				if o.sagaRepo != nil {
					sagaID := s.ID
					if err := o.saveWithOptimisticLockRetry(s, 3, func(reloaded *saga.Saga) bool {
						// 检查是否已经处于补偿或更终态
						if reloaded.Status == saga.StatusCompensating ||
							reloaded.Status == saga.StatusCompensated ||
							reloaded.Status == saga.StatusCompleted {
							return false
						}
						reloaded.Status = saga.StatusCompensating
						reloaded.UpdatedAt = time.Now()
						return true
					}); err != nil {
						log.Printf("❌ Failed to persist saga %s timeout compensation: %v", sagaID, err)
					}
				}

				// 触发补偿
				sagaCtx := &handlers.SagaEventHandlerContext{
					Ctx:           o.ctx,
					KafkaProducer: o.kafkaProducer,
					SagaRepo:      o.sagaRepo,
					InstanceID:    o.instanceID,
					// 其他字段对于超时补偿不是必需的
					EventData:     nil,
					BusinessEvent: nil,
					Sagas:         o.sagas,
					SagasMutex:    &o.sagasMutex,
				}
				handlers.TriggerSagaCompensation(sagaCtx, s, failedStepIndex)
				continue
			}
		}
		s.Mu.Unlock()
	}
}
