package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	consumer_model "orchestrator_service/kafka/consumer/model"
	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
	"orchestrator_service/templates"
)

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
func HandleSagaStartEvent(ctx context.Context, businessEvent *consumer_model.BusinessEvent, kafkaProducer *producer.KafkaProducer, sagas map[string]*saga.Saga, sagasMutex *sync.RWMutex) error {
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
	newSagaID := templates.GetEventID(businessEvent.CommonParams.EventName)

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

	// 添加到Saga集合中
	sagasMutex.Lock()
	sagas[startData.SagaID] = sagaInstance
	sagasMutex.Unlock()

	sagaInstance.Mu.Lock()
	sagaInstance.Status = saga.StatusRunning
	sagaInstance.Mu.Unlock()

	// 发送Saga开始事件
	if err := kafkaProducer.SendEvent(ctx, "saga-events", saga.EventTypeSagaInitiated, sagaInstance.ID, nil); err != nil {
		log.Printf("❌ Failed to send saga initiated event: %v", err)
		// 继续尝试执行步骤
	}

	// 开始执行第一个步骤
	return executeNextSteps(ctx, sagaInstance, kafkaProducer)
}

// executeNextSteps 根据执行模式执行下一个或多个步骤
func executeNextSteps(ctx context.Context, sagaInstance *saga.Saga, kafkaProducer *producer.KafkaProducer) error {
	// 从context中获取执行模式
	executionMode, _ := sagaInstance.Context["execution_mode"].(string)

	// 根据执行模式分发执行逻辑
	switch executionMode {
	case saga.ExecutionModeSerial:
		// 串行模式：只执行第一步
		return executeSequentialStep(ctx, sagaInstance, kafkaProducer, 0)

	case saga.ExecutionModeParallel:
		// 并行模式：同时执行所有步骤
		return executeParallelSteps(ctx, sagaInstance, kafkaProducer)
	default:
		// 未知模式直接报错
		return fmt.Errorf("unknown execution mode: %s", executionMode)
	}
}

// executeSequentialStep 执行串行步骤
func executeSequentialStep(ctx context.Context, sagaInstance *saga.Saga, kafkaProducer *producer.KafkaProducer, stepIndex int) error {
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
func executeParallelSteps(ctx context.Context, sagaInstance *saga.Saga, kafkaProducer *producer.KafkaProducer) error {
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
			for retry := range maxSendRetries {
				if err := kafkaProducer.SendEvent(ctx, s.Name, saga.EventTypeStepExecute, sagaInstance.ID, stepData); err != nil {
					// 简单的退避策略
					time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond)
				} else {
					// 发送成功，跳出循环
					break
				}
			}
		}(stepIndex, step)
	}

	return nil
}
