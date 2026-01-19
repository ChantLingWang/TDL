package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
)

// HandleStepSuccessEvent 处理步骤成功事件
func HandleStepSuccessEvent(ctx context.Context, data []byte, globalKafkaProducer *producer.KafkaProducer, sagas map[string]*saga.Saga, sagasMutex *sync.RWMutex) error {
	var result saga.StepResultData
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("❌ Failed to unmarshal step success event: %v", err)
		return err
	}

	// 获取Saga实例
	sagasMutex.RLock()
	sagaInstance, exists := sagas[result.SagaID]
	sagasMutex.RUnlock()

	if !exists {
		log.Printf("❌ Saga not found: %s", result.SagaID)
		return fmt.Errorf("saga not found: %s", result.SagaID)
	}

	// 更新当前步骤状态
	sagaInstance.Mu.Lock()
	if result.StepIndex < len(sagaInstance.Steps) {
		sagaInstance.Steps[result.StepIndex].Executed = true
		now := time.Now()
		sagaInstance.Steps[result.StepIndex].ExecutedAt = &now
	}
	// 获取当前Saga状态
	currentStatus := sagaInstance.Status
	sagaInstance.Mu.Unlock()

	// 检查Saga是否处于补偿状态
	if currentStatus == saga.StatusCompensating || currentStatus == saga.StatusFailed {
		log.Printf("⚠️ Saga %s is in compensating state, rolling back just-finished step %d", result.SagaID, result.StepIndex)
		TriggerStepCompensation(ctx, sagaInstance, result.StepIndex, globalKafkaProducer)
		return nil
	}

	// 1. 检查是否所有步骤都已完成
	allCompleted := true
	sagaInstance.Mu.Lock()
	for _, s := range sagaInstance.Steps {
		if !s.Executed {
			allCompleted = false
			break
		}
	}

	if allCompleted {
		sagaInstance.Status = saga.StatusCompleted
		sagaInstance.UpdatedAt = time.Now()
	}
	sagaInstance.Mu.Unlock()

	// 2. 如果全部完成，处理 Saga 完成逻辑（无论串行还是并行）
	if allCompleted {
		if err := globalKafkaProducer.SendEvent(ctx, "saga-events", saga.EventTypeSagaCompleted, sagaInstance.ID, nil); err != nil {
			log.Printf("❌ Failed to send saga completed event: %v", err)
			return err
		}
		return nil
	}

	// 3. 如果尚未完成，检查是否需要触发下一步（仅串行模式需要）
	executionMode, _ := sagaInstance.Context["execution_mode"].(string)

	// 并行模式下，各步骤独立执行，不需要在此处触发下一步
	if executionMode == saga.ExecutionModeParallel {
		return nil
	}

	// 串行模式下，触发下一个步骤
	// 但对于串行模式，我们只需要找到"当前完成步骤的下一个"
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
