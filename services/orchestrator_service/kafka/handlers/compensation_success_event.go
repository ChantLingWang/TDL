package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"orchestrator_service/orchestrator/saga"
)

// HandleStepRecoverySuccessEvent 处理步骤补偿成功事件
func HandleStepRecoverySuccessEvent(sagaCtx *SagaEventHandlerContext) error {
	ctx := sagaCtx.Ctx
	data := sagaCtx.EventData
	globalKafkaProducer := sagaCtx.KafkaProducer
	sagas := sagaCtx.Sagas
	sagasMutex := sagaCtx.SagasMutex

	var result saga.StepResultData
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("❌ Failed to unmarshal step compensation success event: %v", err)
		return err
	}

	// 检查分布式锁：只有持有锁的实例才能处理事件
	if sagaCtx.SagaRepo != nil {
		leaseDuration := 30 * time.Second
		renewed, err := sagaCtx.SagaRepo.RenewLock(ctx, result.SagaID, sagaCtx.InstanceID, leaseDuration)
		if err != nil {
			log.Printf("❌ Failed to renew lock for saga %s: %v", result.SagaID, err)
			return fmt.Errorf("failed to renew lock for saga: %w", err)
		}
		if !renewed {
			log.Printf("⚠️ Lock not held by this instance for saga %s, ignoring event", result.SagaID)
			return nil
		}
	}

	// 获取Saga实例
	sagasMutex.RLock()
	sagaInstance, exists := sagas[result.SagaID]
	sagasMutex.RUnlock()

	if !exists {
		log.Printf("❌ Saga not found for compensation success: %s", result.SagaID)
		return fmt.Errorf("saga not found: %s", result.SagaID)
	}

	// 更新当前步骤的补偿状态
	sagaInstance.Mu.Lock()
	if result.StepIndex < len(sagaInstance.Steps) {
		// 这样我们可以通过检查是否所有步骤都是 false 来判断整个 Saga 是否回滚完成
		sagaInstance.Steps[result.StepIndex].Executed = false
	}

	// 检查是否所有步骤都已经回滚（Executed 均为 false）
	allCompensated := true
	for _, step := range sagaInstance.Steps {
		if step.Executed {
			allCompensated = false
			break
		}
	}

	sagaInstance.Mu.Unlock()

	// 持久化步骤补偿状态（使用乐观锁重试）
	compensatedStepIndex := result.StepIndex
	if err := sagaCtx.SaveWithOptimisticLock(sagaInstance, 3, func(s *saga.Saga) bool {
		if compensatedStepIndex < len(s.Steps) {
			s.Steps[compensatedStepIndex].Executed = false
		}
		return true
	}); err != nil {
		log.Printf("❌ Failed to persist saga %s compensation progress: %v", sagaInstance.ID, err)
	}

	if !allCompensated {
		log.Printf("Saga %s compensation in progress (step %d compensated)", sagaInstance.ID, result.StepIndex)
		return nil
	}

	// 设置状态为已补偿
	if !sagaInstance.SetStatus(saga.StatusCompensated) {
		log.Printf("⚠️ Failed to set saga %s status to compensated", sagaInstance.ID)
	}

	// 发送 Saga 完全补偿的事件（用于监控/日志）
	if err := globalKafkaProducer.SendEvent(ctx, "saga-events", saga.EventTypeSagaCompensated, sagaInstance.ID, nil); err != nil {
		log.Printf("Failed to send saga compensated event: %v", err)
		return err
	}

	// 从内存中删除 Saga 实例
	sagasMutex.Lock()
	delete(sagas, sagaInstance.ID)
	sagasMutex.Unlock()

	// 释放分布式锁
	if sagaCtx.SagaRepo != nil {
		released, err := sagaCtx.SagaRepo.ReleaseLock(ctx, sagaInstance.ID, sagaCtx.InstanceID)
		if err != nil {
			log.Printf("⚠️ Failed to release lock for saga %s: %v", sagaInstance.ID, err)
		} else if !released {
			log.Printf("⚠️ Lock not held by this instance for saga %s during cleanup", sagaInstance.ID)
		}
	}

	// 从持久化存储中删除
	if sagaCtx.SagaRepo != nil {
		if err := sagaCtx.SagaRepo.Delete(ctx, sagaInstance.ID); err != nil {
			log.Printf("❌ Failed to delete saga %s from storage: %v", sagaInstance.ID, err)
		}
	}

	log.Printf("Saga %s fully compensated and cleaned up", sagaInstance.ID)

	return nil
}
