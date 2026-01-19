package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
)

// HandleStepRecoverySuccessEvent 处理步骤补偿成功事件
func HandleStepRecoverySuccessEvent(ctx context.Context, data []byte, globalKafkaProducer *producer.KafkaProducer, sagas map[string]*saga.Saga, sagasMutex *sync.RWMutex) error {
	var result saga.StepResultData
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("❌ Failed to unmarshal step compensation success event: %v", err)
		return err
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

	if !allCompensated {
		log.Printf("Saga %s compensation in progress (step %d compensated)", sagaInstance.ID, result.StepIndex)
		return nil
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

	return nil
}
