package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"orchestrator_service/kafka/deadlinequeue"
	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
)

// HandleStepRecoveryFailureEvent 处理步骤补偿失败事件
func HandleStepRecoveryFailureEvent(ctx context.Context, data []byte, globalKafkaProducer *producer.KafkaProducer, sagas map[string]*saga.Saga, sagasMutex *sync.RWMutex) error {
	var result saga.StepResultData
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("❌ Failed to unmarshal step compensation failure event: %v. Data: %s", err, string(data))
		return err
	}

	// 获取Saga实例
	sagasMutex.RLock()
	sagaInstance, exists := sagas[result.SagaID]
	sagasMutex.RUnlock()

	if !exists {
		log.Printf("❌ Saga not found for compensation retry: %s", result.SagaID)
		return fmt.Errorf("saga not found: %s", result.SagaID)
	}

	// 获取需要重试的步骤
	sagaInstance.Mu.Lock()
	if result.StepIndex >= len(sagaInstance.Steps) {
		sagaInstance.Mu.Unlock()
		return fmt.Errorf("invalid step index %d for saga %s", result.StepIndex, result.SagaID)
	}
	step := sagaInstance.Steps[result.StepIndex]
	sagaInstance.Mu.Unlock()

	// 启动协程进行有限重试
	go func() {
		retryCount := 0
		maxRetries := 10 // 最大重试次数
		retryInterval := 2 * time.Second
		maxInterval := 1 * time.Minute

		for {
			retryCount++

			// 检查是否超过最大重试次数
			if retryCount > maxRetries {
				log.Printf("Max retries (%d) reached for saga %s step %d. Sending to DLQ.", maxRetries, sagaInstance.ID, result.StepIndex)

				// 发送到死信队列
				err := deadlinequeue.SendToDLQ(
					context.Background(),
					globalKafkaProducer,
					sagaInstance.ID,
					result.StepIndex,
					step.ServiceName,
					fmt.Sprintf("Compensation failed after %d retries. Last error: %s", maxRetries, result.Error),
					step.Data,
				)

				if err != nil {
					log.Printf("Failed to send to DLQ: %v", err)
				}

				// 停止重试
				return
			}

			// 构建补偿数据
			compensationData := saga.StepExecuteData{
				SagaID:        sagaInstance.ID,
				StepIndex:     result.StepIndex,
				Step:          &step,
				CorrelationID: fmt.Sprintf("%s_compensation_retry_%d_%d", sagaInstance.ID, result.StepIndex, retryCount),
				Parameters:    step.Data,
			}

			// 检查Saga是否已被删除（例如已被其他线程处理完成）
			sagasMutex.RLock()
			_, sagaStillExists := sagas[sagaInstance.ID]
			sagasMutex.RUnlock()

			if !sagaStillExists {
				return
			}

			// 尝试发送补偿事件
			// 使用 step.ServiceName 作为 Topic，因为服务名通常对应其监听的 Topic
			err := globalKafkaProducer.SendEvent(context.Background(), step.ServiceName, saga.EventTypeStepCompensate, sagaInstance.ID, compensationData)
			if err == nil {
				return // 发送成功，退出重试循环
			}

			log.Printf("❌ Failed to resend compensation event: %v. Retrying in %v (Attempt %d/%d)...", err, retryInterval, retryCount, maxRetries)

			// 等待下一次重试
			time.Sleep(retryInterval)

			// 指数退避，但设置上限
			retryInterval *= 2
			if retryInterval > maxInterval {
				retryInterval = maxInterval
			}
		}
	}()

	return nil
}
