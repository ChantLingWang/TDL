package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
)

// HandleStepSuccessEvent 处理步骤成功事件
func HandleStepSuccessEvent(sagaCtx *SagaEventHandlerContext) error {
	ctx := sagaCtx.Ctx
	data := sagaCtx.EventData
	globalKafkaProducer := sagaCtx.KafkaProducer
	sagas := sagaCtx.Sagas
	sagasMutex := sagaCtx.SagasMutex

	var result saga.StepResultData
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("❌ Failed to unmarshal step success event: %v", err)
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
		log.Printf("❌ Saga not found: %s", result.SagaID)
		return fmt.Errorf("saga not found: %s", result.SagaID)
	}

	// 使用单次加锁处理所有状态检查和更新，避免 TOCTOU 竞争
	sagaInstance.Mu.Lock()

	// 检查步骤索引有效性
	if result.StepIndex >= len(sagaInstance.Steps) {
		sagaInstance.Mu.Unlock()
		log.Printf("❌ Invalid step index %d for saga %s", result.StepIndex, result.SagaID)
		return fmt.Errorf("invalid step index: %d", result.StepIndex)
	}

	// 检查Saga是否处于补偿状态
	currentStatus := sagaInstance.Status
	if currentStatus == saga.StatusCompensating || currentStatus == saga.StatusFailed {
		// 检查该步骤是否已经在补偿中，防止重复补偿
		if !sagaInstance.Steps[result.StepIndex].Compensating {
			sagaInstance.Steps[result.StepIndex].Compensating = true
			sagaInstance.Steps[result.StepIndex].Executed = true // 标记为已执行，以便补偿
			now := time.Now()
			sagaInstance.Steps[result.StepIndex].ExecutedAt = &now
			sagaInstance.Mu.Unlock()

			log.Printf("⚠️ Saga %s is in compensating state, rolling back just-finished step %d", result.SagaID, result.StepIndex)
			TriggerStepCompensation(sagaCtx, sagaInstance, result.StepIndex)
		} else {
			sagaInstance.Mu.Unlock()
			log.Printf("⚠️ Step %d of saga %s is already being compensated, skipping", result.StepIndex, result.SagaID)
		}
		return nil
	}

	// 更新当前步骤状态
	sagaInstance.Steps[result.StepIndex].Executed = true
	now := time.Now()
	sagaInstance.Steps[result.StepIndex].ExecutedAt = &now

	// 检查是否所有步骤都已完成
	allCompleted := true
	for _, s := range sagaInstance.Steps {
		if !s.Executed {
			allCompleted = false
			break
		}
	}

	// 获取执行模式
	executionMode, _ := sagaInstance.Context["execution_mode"].(string)
	sagaID := sagaInstance.ID

	if allCompleted {
		// 使用内部方法设置状态（在持有锁的情况下调用公开方法会死锁）
		sagaInstance.Status = saga.StatusCompleted
		sagaInstance.UpdatedAt = time.Now()
		sagaInstance.Version++
	}

	sagaInstance.Mu.Unlock()

	// 持久化Saga状态变更（使用乐观锁重试）
	stepIndexForPersist := result.StepIndex
	allCompletedForPersist := allCompleted
	if err := sagaCtx.SaveWithOptimisticLock(sagaInstance, 3, func(s *saga.Saga) bool {
		// 在重试时重新应用变更
		if stepIndexForPersist < len(s.Steps) {
			s.Steps[stepIndexForPersist].Executed = true
			now := time.Now()
			s.Steps[stepIndexForPersist].ExecutedAt = &now
		}
		if allCompletedForPersist {
			s.Status = saga.StatusCompleted
			s.UpdatedAt = time.Now()
		}
		return true
	}); err != nil {
		log.Printf("❌ Failed to persist saga %s: %v", sagaID, err)
	}

	// 如果全部完成，处理 Saga 完成逻辑
	if allCompleted {
		if err := globalKafkaProducer.SendEvent(ctx, "saga-events", saga.EventTypeSagaCompleted, sagaID, nil); err != nil {
			log.Printf("❌ Failed to send saga completed event: %v", err)
			return err
		}

		// 清理已完成的 Saga，防止内存泄漏
		sagasMutex.Lock()
		delete(sagas, sagaID)
		sagasMutex.Unlock()

		// 释放分布式锁
		if sagaCtx.SagaRepo != nil {
			released, err := sagaCtx.SagaRepo.ReleaseLock(ctx, sagaID, sagaCtx.InstanceID)
			if err != nil {
				log.Printf("⚠️ Failed to release lock for saga %s: %v", sagaID, err)
			} else if !released {
				log.Printf("⚠️ Lock not held by this instance for saga %s during cleanup", sagaID)
			}
		}

		// 从持久化存储中删除
		if sagaCtx.SagaRepo != nil {
			if err := sagaCtx.SagaRepo.Delete(ctx, sagaID); err != nil {
				log.Printf("❌ Failed to delete saga %s from storage: %v", sagaID, err)
			}
		}
		log.Printf("Saga %s completed and cleaned up", sagaID)

		return nil
	}

	// 并行模式下，各步骤独立执行，不需要在此处触发下一步
	if executionMode == saga.ExecutionModeParallel {
		return nil
	}

	// 串行模式下，触发下一个步骤
	nextStepIndex := result.StepIndex + 1
	if nextStepIndex < len(sagaInstance.Steps) {
		triggerNextStep(ctx, sagaInstance, nextStepIndex, globalKafkaProducer)
	}

	return nil
}

// triggerNextStep 触发指定步骤执行
func triggerNextStep(ctx context.Context, sagaInstance *saga.Saga, stepIndex int, kafkaProducer *producer.KafkaProducer) {
	sagaInstance.Mu.Lock()
	step := sagaInstance.Steps[stepIndex]
	sagaInstance.CurrentStep = stepIndex
	sagaInstance.Mu.Unlock()

	stepData := saga.StepExecuteData{
		SagaID:        sagaInstance.ID,
		StepIndex:     stepIndex,
		Step:          &step,
		CorrelationID: fmt.Sprintf("%s_step_%d", sagaInstance.ID, stepIndex),
		Parameters:    step.Data,
	}

	if err := kafkaProducer.SendEvent(ctx, step.Name, saga.EventTypeStepExecute, sagaInstance.ID, stepData); err != nil {
		log.Printf("❌ Failed to send step execute event: %v", err)
	}
}
