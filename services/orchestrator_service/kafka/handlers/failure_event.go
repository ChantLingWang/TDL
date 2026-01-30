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
		// 标记为已执行，虽然是失败的
		sagaInstance.Steps[result.StepIndex].Executed = true
		now := time.Now()
		sagaInstance.Steps[result.StepIndex].ExecutedAt = &now
	}
	// 设置状态为补偿中
	sagaInstance.Status = saga.StatusCompensating
	sagaInstance.Mu.Unlock()

	// 统一触发补偿逻辑：无论之前是串行还是并行执行，
	// 只要步骤被执行过（Executed=true），就需要并发地触发补偿。
	TriggerSagaCompensation(ctx, sagaInstance, result.StepIndex, globalKafkaProducer)

	return nil
}

// TriggerSagaCompensation 触发Saga补偿逻辑
// 遍历所有步骤，检查 Executed 状态，对已执行的步骤并发发送补偿事件
func TriggerSagaCompensation(ctx context.Context, sagaInstance *saga.Saga, failedStepIndex int, kafkaProducer *producer.KafkaProducer) {
	for i := range sagaInstance.Steps {
		// 跳过当前失败的步骤（它通过报错触发了流程，通常不需要补偿，或者由服务自身回滚）
		if i == failedStepIndex {
			continue
		}

		// 安全检查步骤状态
		sagaInstance.Mu.Lock()
		isExecuted := sagaInstance.Steps[i].Executed
		sagaInstance.Mu.Unlock()

		// 只要步骤已执行，就立即触发补偿
		if isExecuted {
			go TriggerStepCompensation(ctx, sagaInstance, i, kafkaProducer)
		}
	}
}
