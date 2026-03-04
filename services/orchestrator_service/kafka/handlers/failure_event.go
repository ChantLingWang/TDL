package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"orchestrator_service/orchestrator/saga"
)

// HandleStepFailureEvent 处理步骤失败事件
func HandleStepFailureEvent(sagaCtx *SagaEventHandlerContext) error {
	ctx := sagaCtx.Ctx
	data := sagaCtx.EventData
	globalKafkaProducer := sagaCtx.KafkaProducer
	sagas := sagaCtx.Sagas
	sagasMutex := sagaCtx.SagasMutex

	var result saga.StepResultData
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("❌ Failed to unmarshal step failure event: %v", err)
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

	// 检查是否可以重试
	sagaInstance.Mu.Lock()
	step := &sagaInstance.Steps[result.StepIndex]

	if step.RetryCount < step.MaxRetries {
		step.RetryCount++
		log.Printf("🔄 Retrying step %d in saga %s (Attempt %d/%d)", result.StepIndex, result.SagaID, step.RetryCount, step.MaxRetries)

		// 准备重试数据
		stepData := saga.StepExecuteData{
			SagaID:        sagaInstance.ID,
			StepIndex:     result.StepIndex,
			Step:          step,
			CorrelationID: fmt.Sprintf("%s_retry_%d", sagaInstance.ID, step.RetryCount),
			Parameters:    step.Data,
		}
		sagaInstance.Mu.Unlock()

		// 持久化重试次数变更（使用乐观锁重试）
		retryStepIndex := result.StepIndex
		retryCount := step.RetryCount
		if err := sagaCtx.SaveWithOptimisticLock(sagaInstance, 3, func(s *saga.Saga) bool {
			if retryStepIndex < len(s.Steps) {
				s.Steps[retryStepIndex].RetryCount = retryCount
			}
			return true
		}); err != nil {
			log.Printf("❌ Failed to persist saga %s retry: %v", sagaInstance.ID, err)
		}

		// 发送重试指令
		if err := globalKafkaProducer.SendEvent(ctx, step.Name, saga.EventTypeStepExecute, sagaInstance.ID, stepData); err != nil {
			log.Printf("❌ Failed to send retry event for step %d: %v", result.StepIndex, err)
		}
		return nil
	}
	sagaInstance.Mu.Unlock()

	// 重试次数已用尽，进入补偿流程
	log.Printf("❌ Step %d failed after %d retries in saga %s: %s", result.StepIndex, step.MaxRetries, result.SagaID, result.Error)

	// 更新当前步骤状态
	sagaInstance.Mu.Lock()
	if result.StepIndex < len(sagaInstance.Steps) {
		sagaInstance.Steps[result.StepIndex].ExecutionLog = result.Error
		sagaInstance.Steps[result.StepIndex].ExecutionData = result.OutputData
		// 失败的步骤不标记为 Executed，因为失败的步骤不需要补偿
		now := time.Now()
		sagaInstance.Steps[result.StepIndex].ExecutedAt = &now
	}
	sagaInstance.Mu.Unlock()

	// 使用 SetStatus 方法设置状态，确保状态转换验证
	if !sagaInstance.SetStatus(saga.StatusCompensating) {
		log.Printf("⚠️ Failed to set saga %s status to compensating (current status may not allow this transition)", sagaInstance.ID)
		// 如果状态转换失败，可能已经处于补偿或完成状态，不需要重复触发补偿
		return nil
	}

	// 持久化补偿状态（使用乐观锁重试）
	failedStepIdx := result.StepIndex
	failedError := result.Error
	failedOutput := result.OutputData
	if err := sagaCtx.SaveWithOptimisticLock(sagaInstance, 3, func(s *saga.Saga) bool {
		// 检查是否已经处于补偿或更终态
		if s.Status == saga.StatusCompensated || s.Status == saga.StatusCompleted {
			return false
		}
		// 更新失败步骤信息
		if failedStepIdx < len(s.Steps) {
			s.Steps[failedStepIdx].ExecutionLog = failedError
			s.Steps[failedStepIdx].ExecutionData = failedOutput
		}
		s.Status = saga.StatusCompensating
		return true
	}); err != nil {
		log.Printf("❌ Failed to persist saga %s compensating status: %v", sagaInstance.ID, err)
	}

	// 统一触发补偿逻辑：无论之前是串行还是并行执行，
	// 只要步骤被执行过（Executed=true），就需要并发地触发补偿。
	TriggerSagaCompensation(sagaCtx, sagaInstance, result.StepIndex)

	return nil
}

// TriggerSagaCompensation 触发Saga补偿逻辑
// 遍历所有步骤，检查 Executed 状态，对已执行的步骤并发发送补偿事件
func TriggerSagaCompensation(sagaCtx *SagaEventHandlerContext, sagaInstance *saga.Saga, failedStepIndex int) {
	for i := range sagaInstance.Steps {
		// 跳过当前失败的步骤（它通过报错触发了流程，通常不需要补偿，或者由服务自身回滚）
		if i == failedStepIndex {
			continue
		}

		// 安全检查步骤状态，并标记为正在补偿以防止重复补偿
		sagaInstance.Mu.Lock()
		isExecuted := sagaInstance.Steps[i].Executed
		isCompensating := sagaInstance.Steps[i].Compensating

		// 只有已执行且尚未开始补偿的步骤才需要触发补偿
		if isExecuted && !isCompensating {
			sagaInstance.Steps[i].Compensating = true
			sagaInstance.Mu.Unlock()
			go TriggerStepCompensation(sagaCtx, sagaInstance, i)
		} else {
			sagaInstance.Mu.Unlock()
		}
	}
}
