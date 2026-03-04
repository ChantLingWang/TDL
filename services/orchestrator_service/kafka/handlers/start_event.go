package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"orchestrator_service/orchestrator/saga"
	"orchestrator_service/templates"

	"github.com/bwmarrin/snowflake"
)

// 雪花算法生成器实例
var snowflakeNode *snowflake.Node

// InitSnowflake 初始化雪花算法生成器
func InitSnowflake(nodeID int64) error {
	var err error
	snowflakeNode, err = snowflake.NewNode(nodeID)
	if err != nil {
		return fmt.Errorf("failed to initialize snowflake node: %w", err)
	}
	return nil
}

// GetEventID 获取事务ID - 使用雪花算法生成可查询且不重复的数字事务ID
func GetEventID() string {
	if snowflakeNode == nil {
		// 如果未初始化，尝试使用默认节点ID 1初始化（仅作为兜底）
		_ = InitSnowflake(1)
	}
	return fmt.Sprintf("%d", snowflakeNode.Generate().Int64())
}

// mergeData 合并数据：按优先级递增顺序覆盖
func mergeData(templateData map[string]any, inputData map[string]any) map[string]any {
	result := make(map[string]any)

	// 第一步：加载模板默认配置作为基础
	for k, v := range templateData {
		result[k] = v
	}

	// 第二步：将传入的步骤数据合并（覆盖同名key）
	for k, v := range inputData {
		result[k] = v
	}

	return result
}

// HandleSagaStartEvent 处理Saga启动事件
func HandleSagaStartEvent(sagaCtx *SagaEventHandlerContext) error {
	ctx := sagaCtx.Ctx
	businessEvent := sagaCtx.BusinessEvent
	kafkaProducer := sagaCtx.KafkaProducer
	sagas := sagaCtx.Sagas
	sagasMutex := sagaCtx.SagasMutex

	// 解析为字典格式，key为步骤topic/name，value为步骤数据
	var dataMap map[string]map[string]any
	// 尝试解析为 map[string]map[string]any
	if err := json.Unmarshal(businessEvent.Data, &dataMap); err != nil {
		log.Printf("❌ Failed to unmarshal saga start event data as map: %v", err)
		return err
	}

	// 根据event_name加载模板，获取步骤定义
	template, err := templates.LoadTemplate(businessEvent.CommonParams.EventName)
	if err != nil {
		log.Printf("❌ Failed to load template for event name %s: %v", businessEvent.CommonParams.EventName, err)
		return err
	}

	// 初始化SagaStartData
	newSagaID := GetEventID()

	startData := saga.SagaStartData{
		SagaID: newSagaID,                // 使用新生成的唯一SagaID
		Steps:  make([]saga.SagaStep, 0), // 步骤列表
		// Parameters 字段在此流程中未使用，留空即可
	}

	// 根据模板和传入的数据初始化Steps
	startData.Steps = make([]saga.SagaStep, 0)
	for i, templateStep := range template.Steps {
		// templateStep 现在是 StepData 结构体
		templateStepData := templateStep.Data
		if templateStepData == nil {
			templateStepData = make(map[string]any)
		}

		// 获取传入的步骤数据
		var inputStepData map[string]any

		// 优先使用 Topic 匹配
		if val, ok := dataMap[templateStep.Topic]; ok {
			inputStepData = val
		} else if val, ok := dataMap[templateStep.Name]; ok {
			// 其次尝试 Name 匹配
			inputStepData = val
		} else {
			// 没找到则为空
			inputStepData = make(map[string]any)
		}

		// 合并数据：模板数据 -> 传入数据
		mergedData := mergeData(templateStepData, inputStepData)

		// 确定最大重试次数
		maxRetries := templateStep.MaxRetries
		if maxRetries <= 0 {
			maxRetries = 3 // 默认最大重试次数
		}

		// 确定步骤名称：优先使用模板中的Name，如果为空则使用Topic
		stepName := templateStep.Name
		if stepName == "" {
			stepName = templateStep.Topic
		}

		// 创建新的SagaStep
		step := saga.SagaStep{
			ID:               fmt.Sprintf("%s_%d", templateStep.Topic, i), // 使用 Topic_Index 作为唯一ID，防止同一Topic多次调用冲突
			Name:             stepName,                                    // 使用处理后的 Name
			ServiceName:      templateStep.Topic,
			Action:           templateStep.Topic,
			Data:             mergedData,
			Order:            i,
			CompensateAction: templateStep.CompensateAction,
			Executed:         false,
			RetryCount:       0,
			MaxRetries:       maxRetries,
		}

		startData.Steps = append(startData.Steps, step)
	}

	// 创建新的Saga实例
	sagaInstance := saga.NewSaga(startData.SagaID, startData.Steps)

	// 传递执行模式（用于控制串行/并行执行）
	sagaInstance.Context["execution_mode"] = businessEvent.CommonParams.ExecutionMode

	// 检查空步骤列表
	if len(startData.Steps) == 0 {
		log.Printf("⚠️ Saga %s has no steps, marking as completed", startData.SagaID)
		sagaInstance.SetStatus(saga.StatusCompleted)
		if err := kafkaProducer.SendEvent(ctx, "saga-events", saga.EventTypeSagaCompleted, sagaInstance.ID, nil); err != nil {
			log.Printf("❌ Failed to send saga completed event: %v", err)
		}
		return nil
	}

	// 添加到Saga集合中
	sagasMutex.Lock()
	sagas[startData.SagaID] = sagaInstance
	sagasMutex.Unlock()

	// 使用 SetStatus 方法设置状态，确保状态转换验证
	if !sagaInstance.SetStatus(saga.StatusRunning) {
		log.Printf("❌ Failed to set saga %s status to running", sagaInstance.ID)
		return fmt.Errorf("failed to set saga status to running")
	}

	// 持久化新创建的Saga
	if sagaCtx.SagaRepo != nil {
		if err := sagaCtx.SagaRepo.Create(ctx, sagaInstance); err != nil {
			log.Printf("❌ Failed to persist saga %s: %v", sagaInstance.ID, err)
			return fmt.Errorf("failed to persist saga: %w", err)
		}

		// 获取分布式锁，确保当前实例负责处理此Saga
		leaseDuration := 30 * time.Second
		acquired, err := sagaCtx.SagaRepo.AcquireLock(ctx, sagaInstance.ID, sagaCtx.InstanceID, leaseDuration)
		if err != nil {
			log.Printf("❌ Failed to acquire lock for saga %s: %v", sagaInstance.ID, err)
			return fmt.Errorf("failed to acquire lock for saga: %w", err)
		}
		if !acquired {
			log.Printf("❌ Failed to acquire lock for saga %s (lock already held)", sagaInstance.ID)
			return fmt.Errorf("failed to acquire lock for saga: lock already held")
		}
	}

	// 发送Saga开始事件
	if err := kafkaProducer.SendEvent(ctx, "saga-events", saga.EventTypeSagaInitiated, sagaInstance.ID, nil); err != nil {
		log.Printf("❌ Failed to send saga initiated event: %v", err)
		// 继续尝试执行步骤
	}

	// 开始执行第一个步骤
	return executeNextSteps(sagaCtx, sagaInstance)
}

// executeNextSteps 根据执行模式执行下一个或多个步骤
func executeNextSteps(sagaCtx *SagaEventHandlerContext, sagaInstance *saga.Saga) error {
	// 从context中获取执行模式
	executionMode, _ := sagaInstance.Context["execution_mode"].(string)

	// 根据执行模式分发执行逻辑
	switch executionMode {
	case saga.ExecutionModeSerial:
		// 串行模式：只执行第一步
		return executeSequentialStep(sagaCtx, sagaInstance, 0)

	case saga.ExecutionModeParallel:
		// 并行模式：同时执行所有步骤
		return executeParallelSteps(sagaCtx, sagaInstance)
	default:
		// 未知模式直接报错
		return fmt.Errorf("unknown execution mode: %s", executionMode)
	}
}

// executeSequentialStep 执行串行步骤
func executeSequentialStep(sagaCtx *SagaEventHandlerContext, sagaInstance *saga.Saga, stepIndex int) error {
	ctx := sagaCtx.Ctx
	kafkaProducer := sagaCtx.KafkaProducer
	if stepIndex >= len(sagaInstance.Steps) {
		sagaInstance.SetStatus(saga.StatusCompleted)
		// 完成事件发送到默认Topic（saga-events）
		if err := kafkaProducer.SendEvent(ctx, "saga-events", saga.EventTypeSagaCompleted, sagaInstance.ID, nil); err != nil {
			log.Printf("❌ Failed to send saga completed event: %v", err)
		}
		return nil
	}

	step := sagaInstance.Steps[stepIndex]
	sagaInstance.Mu.Lock()
	sagaInstance.CurrentStep = stepIndex
	sagaInstance.Mu.Unlock()

	// 构建步骤执行数据
	stepData := saga.StepExecuteData{
		SagaID:        sagaInstance.ID,
		StepIndex:     stepIndex,
		Step:          &step,
		CorrelationID: fmt.Sprintf("%s_step_%d", sagaInstance.ID, stepIndex),
		Parameters:    step.Data, // 已经合并了所有数据
	}

	// 发送到步骤对应的Topic（步骤名即为Topic名）
	return kafkaProducer.SendEvent(ctx, step.Name, saga.EventTypeStepExecute, sagaInstance.ID, stepData)
}

// executeParallelSteps 执行并行步骤
func executeParallelSteps(sagaCtx *SagaEventHandlerContext, sagaInstance *saga.Saga) error {
	ctx := sagaCtx.Ctx
	kafkaProducer := sagaCtx.KafkaProducer
	// 使用 channel 收集发送失败的步骤
	type sendResult struct {
		stepIndex int
		err       error
	}
	resultCh := make(chan sendResult, len(sagaInstance.Steps))

	// 直接遍历所有步骤并行执行
	for i := range sagaInstance.Steps {
		// 捕获变量
		step := sagaInstance.Steps[i]
		stepIndex := i

		go func(idx int, s saga.SagaStep) {
			// 创建步骤执行数据
			stepData := saga.StepExecuteData{
				SagaID:        sagaInstance.ID,
				StepIndex:     idx,
				Step:          &s,
				CorrelationID: fmt.Sprintf("%s_step_%d", sagaInstance.ID, idx),
				Parameters:    s.Data,
			}

			// 发送事件到Kafka（步骤名即为Topic名）
			// 简单的重试逻辑：失败后重试3次
			maxSendRetries := 3
			var lastErr error
			for retry := range maxSendRetries {
				if err := kafkaProducer.SendEvent(ctx, s.Name, saga.EventTypeStepExecute, sagaInstance.ID, stepData); err != nil {
					lastErr = err
					// 简单的退避策略
					time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond)
				} else {
					// 发送成功
					resultCh <- sendResult{stepIndex: idx, err: nil}
					return
				}
			}
			// 所有重试都失败
			log.Printf("❌ Failed to send step %d after %d retries: %v", idx, maxSendRetries, lastErr)
			resultCh <- sendResult{stepIndex: idx, err: lastErr}
		}(stepIndex, step)
	}

	// 收集所有结果
	var failedSteps []int
	for range sagaInstance.Steps {
		result := <-resultCh
		if result.err != nil {
			failedSteps = append(failedSteps, result.stepIndex)
		}
	}

	// 如果有步骤发送失败，触发补偿
	if len(failedSteps) > 0 {
		log.Printf("❌ Saga %s: %d steps failed to send, triggering compensation", sagaInstance.ID, len(failedSteps))
		sagaInstance.SetStatus(saga.StatusCompensating)
		// 对已成功发送的步骤触发补偿（由于是并行发送，可能有些已经开始执行）
		// 注意：这里使用 -1 表示没有特定的失败步骤
		TriggerSagaCompensation(sagaCtx, sagaInstance, -1)
		return fmt.Errorf("failed to send %d steps", len(failedSteps))
	}

	return nil
}
