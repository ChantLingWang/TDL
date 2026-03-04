package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"infrastructure_sdk/kafka"
	"orchestrator_service/orchestrator/saga"
)

// 用于跟踪正在进行补偿重试的步骤，防止重复启动重试 goroutine
var (
	compensationRetryTracker      = make(map[string]bool) // key: sagaID_stepIndex
	compensationRetryTrackerMutex sync.Mutex
)

// HandleStepRecoveryFailureEvent 处理步骤补偿失败事件
func HandleStepRecoveryFailureEvent(sagaCtx *SagaEventHandlerContext) error {
	ctx := sagaCtx.Ctx
	_ = ctx
	data := sagaCtx.EventData
	globalKafkaProducer := sagaCtx.KafkaProducer
	sagas := sagaCtx.Sagas
	sagasMutex := sagaCtx.SagasMutex

	var result saga.StepResultData
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("❌ Failed to unmarshal step compensation failure event: %v. Data: %s", err, string(data))
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

	// 检查是否已经有重试 goroutine 在运行，防止重复启动
	retryKey := fmt.Sprintf("%s_%d", sagaInstance.ID, result.StepIndex)
	compensationRetryTrackerMutex.Lock()
	if compensationRetryTracker[retryKey] {
		compensationRetryTrackerMutex.Unlock()
		log.Printf("⚠️ Compensation retry already in progress for saga %s step %d, skipping duplicate", sagaInstance.ID, result.StepIndex)
		return nil
	}
	compensationRetryTracker[retryKey] = true
	compensationRetryTrackerMutex.Unlock()

	// 启动协程进行有限重试
	go func() {
		// 确保在 goroutine 退出时清理跟踪状态
		defer func() {
			compensationRetryTrackerMutex.Lock()
			delete(compensationRetryTracker, retryKey)
			compensationRetryTrackerMutex.Unlock()
		}()
		retryCount := 0
		maxRetries := 10 // 最大重试次数
		retryInterval := 2 * time.Second
		maxInterval := 1 * time.Minute

		for {
			retryCount++

			// 检查是否超过最大重试次数
			if retryCount > maxRetries {
				log.Printf("Max retries (%d) reached for saga %s step %d. Sending to DLQ.", maxRetries, sagaInstance.ID, result.StepIndex)

				// 构造 DLQ Payload
				// 我们需要重构一个"补偿指令"事件，以便后续可以重试
				// 使用 saga.EventTypeStepCompensate 作为事件类型，表明这是一个补偿操作
				reconstructedEvent, _ := kafka.NewBusinessEvent(
					saga.EventTypeStepCompensate,
					"Step Compensation Retry",
					sagaInstance.ID,
					step.Data,
				)

				dlqPayload := kafka.NewDLQPayload(
					step.ServiceName, // 假设 ServiceName 对应 topic
					reconstructedEvent,
					fmt.Sprintf("Compensation failed after %d retries. Last error: %s", maxRetries, result.Error),
				)
				dlqPayload.Service = "orchestrator_service"

				// 发送到死信队列 (Topic: saga-dlq)
				err := globalKafkaProducer.SendEvent(
					context.Background(),
					saga.TopicSagaDLQ,
					kafka.DLQEventType,
					sagaInstance.ID,
					dlqPayload,
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
