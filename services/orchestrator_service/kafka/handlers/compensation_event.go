package handlers

import (
	"context"
	"fmt"
	"log"

	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
)

// TriggerStepCompensation 触发指定步骤的补偿操作
// 这是一个通用函数，用于在各种需要回滚的场景下（如并行执行中的部分失败、整体Saga失败等）调用
func TriggerStepCompensation(ctx context.Context, sagaInstance *saga.Saga, stepIndex int, kafkaProducer *producer.KafkaProducer) {
	sagaInstance.Mu.Lock()
	if stepIndex >= len(sagaInstance.Steps) {
		sagaInstance.Mu.Unlock()
		return
	}
	step := sagaInstance.Steps[stepIndex]
	sagaInstance.Mu.Unlock()

	compensationData := saga.StepExecuteData{
		SagaID:        sagaInstance.ID,
		StepIndex:     stepIndex,
		Step:          &step,
		CorrelationID: fmt.Sprintf("%s_compensation_%d", sagaInstance.ID, stepIndex),
		Parameters:    step.Data, // 通常补偿操作需要原始执行数据作为上下文
	}

	// 发送补偿指令到对应的 Topic
	// 注意：通常补偿指令也是发给原服务的 Topic，由服务内部根据 EventType 区分是执行还是补偿
	if err := kafkaProducer.SendEvent(ctx, step.Name, saga.EventTypeStepCompensate, sagaInstance.ID, compensationData); err != nil {
		log.Printf("❌ Failed to send compensation event for step %d: %v", stepIndex, err)
	}
}
