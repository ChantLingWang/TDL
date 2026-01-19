package saga

import (
	"encoding/json"
	"sync"
	"time"
)

// ========== Saga状态定义 ==========

// SagaStatus Saga事务状态 - 统一在 schemas 包中定义
type SagaStatus string

const (
	ExecutionModeParallel = "parallel"   // 并行执行模式
	ExecutionModeSerial   = "sequential" // 串行执行模式
)

const (
	StatusPending      SagaStatus = "pending"      // 待执行
	StatusRunning      SagaStatus = "running"      // 执行中
	StatusCompleted    SagaStatus = "completed"    // 完成
	StatusFailed       SagaStatus = "failed"       // 失败
	StatusCompensating SagaStatus = "compensating" // 补偿中
	StatusCompensated  SagaStatus = "compensated"  // 已补偿
	StatusCancelled    SagaStatus = "cancelled"    // 已取消
)

// 事件类型字符串常量

// ========== Saga编排器事件定义 ==========

// SagaEventType Saga事件类型常量
const (
	// 事务生命周期事件 - 用于追踪整个Saga流程的状态变化
	EventTypeSagaInitiated    = "saga.initiated"     // Saga实例已创建，流程开始
	EventTypeSagaStepExecuted = "saga.step_executed" // 某个步骤执行成功
	EventTypeSagaStepFailed   = "saga.step_failed"   // 某个步骤执行失败
	EventTypeSagaCompleted    = "saga.completed"     // 整个Saga流程成功完成
	EventTypeSagaCompensated  = "saga.compensated"   // Saga已通过补偿操作回滚
	EventTypeSagaFailed       = "saga.failed"        // Saga流程最终失败
	EventTypeSagaCancelled    = "saga.cancelled"     // Saga被手动取消

	// 步骤执行事件 - 用于单个步骤的执行状态
	EventTypeStepExecute        = "saga.step.execute"         // 指令执行某个步骤
	EventTypeStepExecuteSuccess = "saga.step.execute_success" // 步骤执行成功
	EventTypeStepExecuteFail    = "saga.step.execute_fail"    // 步骤执行失败
	EventTypeStepTimeout        = "saga.step.timeout"         // 步骤执行超时

	// 补偿事件 - 用于处理失败的补偿操作
	EventTypeStepCompensate        = "saga.step.compensate"         // 指令执行补偿操作
	EventTypeStepCompensateSuccess = "saga.step.compensate_success" // 补偿操作成功
	EventTypeStepCompensateFail    = "saga.step.compensate_fail"    // 补偿操作失败

	// 死信队列相关
	EventTypeDLQMessage = "saga.dlq.message" // 死信队列消息事件类型
	TopicSagaDLQ        = "saga-dlq"         // 死信队列Topic名称
)

// OrchestratorEventType 编排器反馈事件类型常量
const (
	// 事务启动事件 - 编排器主动触发的Saga启动
	EventTypeSagaStart = "start-event" // 启动一个新的Saga流程

	// 服务执行结果反馈 - 各服务向编排器反馈执行结果
	EventTypeStepSuccess         = "event-success"         // 步骤执行成功反馈
	EventTypeStepFailed          = "event-failed"          // 步骤执行失败反馈
	EventTypeStepRecoverySuccess = "event-recover-success" // 补偿操作成功反馈
	EventTypeStepRecoveryFail    = "event-recover-fail"    // 补偿操作失败反馈
)

// SagaEvent Saga编排器业务事件 - 描述Saga流程中的所有事件信息
type SagaEvent struct {
	EventType   string          `json:"event_type"`             // 事件类型，如"saga.initiated"、"step.execute"等
	SagaID      string          `json:"saga_id"`                // 关联的Saga实例唯一标识
	StepIndex   int             `json:"step_index,omitempty"`   // 当前步骤索引，用于追踪执行进度
	Data        json.RawMessage `json:"data"`                   // 事件承载的业务数据，如步骤参数、执行结果等
	Timestamp   time.Time       `json:"timestamp"`              // 事件发生的时间戳
	ServiceName string          `json:"service_name,omitempty"` // 产生事件的服务名称，用于追踪消息来源
}

// StepExecuteData 步骤执行数据 - 用于向具体服务发送执行指令的数据结构
type StepExecuteData struct {
	SagaID        string                 `json:"saga_id"`              // 关联的Saga实例标识
	StepIndex     int                    `json:"step_index"`           // 要执行的步骤索引
	Step          *SagaStep              `json:"step"`                 // 步骤的详细信息，包括服务名、执行动作等
	CorrelationID string                 `json:"correlation_id"`       // 关联ID，用于追踪整个Saga流程
	Parameters    map[string]interface{} `json:"parameters,omitempty"` // 执行该步骤所需的业务参数
}

// StepResultData 步骤执行结果数据 - 用于服务向编排器反馈执行结果
type StepResultData struct {
	SagaID     string                 `json:"saga_id"`               // 关联的Saga实例标识
	StepIndex  int                    `json:"step_index"`            // 已执行完成的步骤索引
	Success    bool                   `json:"success"`               // 执行是否成功的标志
	Error      string                 `json:"error,omitempty"`       // 执行失败时的错误信息
	OutputData map[string]interface{} `json:"output_data,omitempty"` // 执行成功时返回的业务数据
	Timestamp  time.Time              `json:"timestamp"`             // 执行完成的时间戳
}

// SagaCompensationData Saga补偿数据 - 用于执行Saga补偿操作的数据结构
type SagaCompensationData struct {
	SagaID          string     `json:"saga_id"`           // 关联的Saga实例标识
	FailedStepIndex int        `json:"failed_step_index"` // 失败的步骤索引，补偿从这里开始
	CompletedSteps  []SagaStep `json:"completed_steps"`   // 已成功完成的步骤列表，需要被补偿回滚
	Reason          string     `json:"reason"`            // 补偿操作的触发原因，如执行失败的具体原因
}

// SagaStartData Saga启动数据 - 用于启动新Saga流程的完整配置信息
type SagaStartData struct {
	SagaID        string                 `json:"saga_id"`              // 新创建Saga实例的唯一标识
	Steps         []SagaStep             `json:"steps"`                // Saga包含的完整步骤列表，定义了执行顺序和每个步骤的详细信息
	Parameters    map[string]interface{} `json:"parameters,omitempty"` // 整个Saga流程所需的业务参数，如用户信息、订单详情等
	ExecutionMode string                 `json:"execution_mode"`       // 执行模式，如"sequential"(顺序执行)、"parallel"(并行执行)等
}

// ========== Saga业务数据结构定义 ==========

// StepResult 步骤执行结果
type StepResult struct {
	Success   bool                   `json:"success"`         // 步骤执行是否成功，true表示成功，false表示失败
	Data      map[string]interface{} `json:"data,omitempty"`  // 步骤执行的返回数据，包含成功时的业务数据
	Error     string                 `json:"error,omitempty"` // 步骤执行失败的错误信息
	StepID    string                 `json:"step_id"`         // 步骤的唯一标识符
	Timestamp time.Time              `json:"timestamp"`       // 步骤执行完成的时间戳
}

// SagaStep 表示Saga的一个步骤
type SagaStep struct {
	ID          string         `json:"id"`           // 步骤唯一标识符
	Name        string         `json:"name"`         // 步骤名称，描述该步骤的作用
	ServiceName string         `json:"service_name"` // 执行该步骤的微服务名称
	Action      string         `json:"action"`       // 具体要执行的操作名称
	Data        map[string]any `json:"data"`         // 步骤执行所需的业务数据
	Order       int            `json:"order"`        // 步骤在Saga中的执行顺序

	// 补偿操作
	CompensateAction string `json:"compensate_action"` // 补偿操作名称，用于步骤失败时回滚

	// 执行结果
	Executed      bool           `json:"executed"`                 // 步骤是否已经执行完成
	ExecutedAt    *time.Time     `json:"executed_at,omitempty"`    // 步骤执行完成的具体时间
	ExecutionLog  string         `json:"execution_log,omitempty"`  // 步骤执行过程的日志信息
	ExecutionData map[string]any `json:"execution_data,omitempty"` // 步骤实际执行时的数据
	RetryCount    int            `json:"retry_count"`              // 该步骤已经重试的次数
	MaxRetries    int            `json:"max_retries"`              // 该步骤允许的最大重试次数
}

// Saga 表示一个Saga事务
type Saga struct {
	ID          string         `json:"id"`                // Saga事务的唯一标识符
	Status      SagaStatus     `json:"status"`            // Saga当前的状态，如pending、running、completed等
	Steps       []SagaStep     `json:"steps"`             // Saga包含的所有步骤列表
	CurrentStep int            `json:"current_step"`      // 当前正在执行的步骤索引
	CreatedAt   time.Time      `json:"created_at"`        // Saga创建时间
	UpdatedAt   time.Time      `json:"updated_at"`        // 最后更新时间
	Context     map[string]any `json:"context,omitempty"` // 上下文数据，存储Saga执行过程中的临时数据

	// 扩展字段
	Version       int    `json:"version,omitempty"`        // Saga版本号，用于乐观锁控制并发修改
	CorrelationID string `json:"correlation_id,omitempty"` // 关联ID，用于跨服务事务链追踪
	TraceID       string `json:"trace_id,omitempty"`       // 全局追踪ID，用于对接全链路监控

	// 统计信息
	CompletedSteps int `json:"completed_steps,omitempty"` // 已成功完成的步骤数量
	FailedSteps    int `json:"failed_steps,omitempty"`    // 失败的步骤数量
	RetryCount     int `json:"retry_count,omitempty"`     // 整个Saga的重试次数
	MaxRetryCount  int `json:"max_retry_count,omitempty"` // Saga允许的最大重试次数

	// 互斥锁保护并发安全（导出字段供外部包使用）
	Mu sync.Mutex // 并发控制锁，确保多线程访问Saga数据的安全性
}

// SagaWithStats 包含统计信息的Saga结构
type SagaWithStats struct {
	*Saga                 // 内嵌Saga结构，包含所有Saga的基础信息
	Progress      float64 `json:"progress_percent"` // Saga执行进度百分比，0.0-100.0
	Duration      string  `json:"duration"`         // Saga已持续时间的字符串表示，如"2m30s"
	DurationMilli int64   `json:"duration_ms"`      // Saga已持续时间的毫秒数
}

// MarshalJSON 自定义JSON序列化方法
func (s *Saga) MarshalJSON() ([]byte, error) {
	progress := 0.0
	if len(s.Steps) > 0 {
		completed := 0
		for _, step := range s.Steps {
			if step.Executed {
				completed++
			}
		}
		progress = float64(completed) / float64(len(s.Steps)) * 100
	}

	type Alias Saga
	return json.Marshal(&struct {
		*Alias
		Progress      float64 `json:"progress_percent"`
		Duration      string  `json:"duration"`
		DurationMilli int64   `json:"duration_ms"`
	}{
		Alias:         (*Alias)(s),
		Progress:      progress,
		Duration:      time.Since(s.CreatedAt).String(),
		DurationMilli: time.Since(s.CreatedAt).Milliseconds(),
	})
}
