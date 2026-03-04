package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	sdk_kafka "infrastructure_sdk/kafka"
	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
)

// SagaEventHandlerContext Saga事件处理器上下文，封装所有必要的参数
type SagaEventHandlerContext struct {
	Ctx           context.Context
	EventData     json.RawMessage          // 原始事件数据 (businessEvent.Data)
	BusinessEvent *sdk_kafka.BusinessEvent // 完整的业务事件对象
	KafkaProducer *producer.KafkaProducer
	Sagas         map[string]*saga.Saga
	SagasMutex    *sync.RWMutex
	SagaRepo      saga.SagaRepository // Saga持久化仓库
	InstanceID    string              // 编排器实例ID，用于分布式锁
}

// SaveWithOptimisticLock 使用乐观锁保存Saga，自动处理版本冲突重试
// updateFn: 更新函数，接收最新的Saga状态，返回是否需要继续保存
// maxRetries: 最大重试次数
//
// 使用示例:
//
//	err := sagaCtx.SaveWithOptimisticLock(sagaInstance, 3, func(s *saga.Saga) bool {
//	    // 检查状态是否仍需要更新
//	    if s.Status == saga.StatusCompleted {
//	        return false // 其他实例已完成，不需要继续
//	    }
//	    // 应用更新
//	    s.Steps[stepIndex].Executed = true
//	    return true
//	})
func (ctx *SagaEventHandlerContext) SaveWithOptimisticLock(
	sagaInstance *saga.Saga,
	maxRetries int,
	updateFn func(s *saga.Saga) bool,
) error {
	if ctx.SagaRepo == nil {
		return nil // 没有配置持久化仓库，直接返回
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 第一次尝试使用传入的实例，后续尝试从数据库重新加载
		var currentSaga *saga.Saga
		if attempt == 0 {
			currentSaga = sagaInstance
		} else {
			// 从数据库重新加载最新状态
			var err error
			currentSaga, err = ctx.SagaRepo.Get(ctx.Ctx, sagaInstance.ID)
			if err != nil {
				if errors.Is(err, saga.ErrSagaNotFound) {
					// Saga 已被删除（可能已完成或被清理）
					log.Printf("⚠️ Saga %s no longer exists, skipping save", sagaInstance.ID)
					return nil
				}
				return fmt.Errorf("failed to reload saga: %w", err)
			}

			// 同步更新内存中的Saga实例版本号
			sagaInstance.Version = currentSaga.Version
		}

		// 调用更新函数检查是否需要继续保存
		if !updateFn(currentSaga) {
			log.Printf("⚠️ Saga %s update skipped by updateFn (attempt %d)", sagaInstance.ID, attempt+1)
			return nil
		}

		// 尝试保存
		err := ctx.SagaRepo.Save(ctx.Ctx, currentSaga)
		if err == nil {
			// 保存成功，同步版本号到内存中的实例
			sagaInstance.Version = currentSaga.Version
			return nil
		}

		// 检查是否是版本冲突
		if errors.Is(err, saga.ErrVersionConflict) {
			if attempt < maxRetries {
				log.Printf("⚠️ Saga %s version conflict (attempt %d/%d), retrying...",
					sagaInstance.ID, attempt+1, maxRetries+1)
				continue
			}
			return fmt.Errorf("saga %s version conflict after %d attempts: %w",
				sagaInstance.ID, maxRetries+1, err)
		}

		// 其他错误直接返回
		return err
	}

	return nil
}
