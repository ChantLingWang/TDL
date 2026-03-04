package saga

import (
	"sync"
	"time"
)

// ========== Saga状态定义 ==========
// NewSaga 创建新的Saga实例
func NewSaga(id string, steps []SagaStep) *Saga {
	return &Saga{
		ID:            id,
		Status:        StatusPending,
		Steps:         steps,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Mu:            sync.Mutex{},
		CurrentStep:   -1,
		Context:       make(map[string]any),
		MaxRetryCount: 3, // 默认最大重试次数
	}
}

// SetStatus 安全设置Saga状态
func (s *Saga) SetStatus(newStatus SagaStatus) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	return s.setStatusLocked(newStatus)
}

// setStatusLocked 在已持有锁的情况下设置状态
func (s *Saga) setStatusLocked(newStatus SagaStatus) bool {
	// 验证状态转换是否合法
	if !s.isValidStatusTransition(s.Status, newStatus) {
		return false
	}

	s.Status = newStatus
	s.UpdatedAt = time.Now()
	s.Version++

	return true
}

// isValidStatusTransition 验证状态转换是否合法
func (s *Saga) isValidStatusTransition(from, to SagaStatus) bool {
	validTransitions := map[SagaStatus][]SagaStatus{
		StatusPending:      {StatusRunning, StatusFailed, StatusCancelled},
		StatusRunning:      {StatusCompleted, StatusFailed, StatusCompensating},
		StatusFailed:       {StatusCompensating, StatusCancelled},
		StatusCompensating: {StatusCompensated, StatusFailed},
		StatusCompensated:  {},
		StatusCancelled:    {},
	}

	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, allowedStatus := range allowed {
		if allowedStatus == to {
			return true
		}
	}

	return false
}

// GetCurrentStep 获取当前步骤
func (s *Saga) GetCurrentStep() *SagaStep {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if s.CurrentStep >= 0 && s.CurrentStep < len(s.Steps) {
		return &s.Steps[s.CurrentStep]
	}
	return nil
}

// AdvanceToNextStep 推进到下一步
func (s *Saga) AdvanceToNextStep() *SagaStep {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// 更新当前步骤状态
	if s.CurrentStep >= 0 && s.CurrentStep < len(s.Steps) {
		s.Steps[s.CurrentStep].Executed = true
		executedAt := time.Now()
		s.Steps[s.CurrentStep].ExecutedAt = &executedAt
	}

	// 移动到下一步
	s.CurrentStep++
	s.UpdatedAt = time.Now()
	s.Version++

	// 更新统计信息
	s.CompletedSteps++

	// 检查是否完成所有步骤
	if s.CurrentStep >= len(s.Steps) {
		s.setStatusLocked(StatusCompleted)
	}

	if s.CurrentStep >= 0 && s.CurrentStep < len(s.Steps) {
		return &s.Steps[s.CurrentStep]
	}
	return nil
}

// MarkStepFailed 标记步骤失败
func (s *Saga) MarkStepFailed(stepIndex int, errorMsg string) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if stepIndex >= 0 && stepIndex < len(s.Steps) {
		// 注意：失败的步骤不标记为 Executed，因为 Executed 用于判断是否需要补偿
		// 只有成功执行的步骤才需要补偿
		s.Steps[stepIndex].ExecutionLog = errorMsg
		executedAt := time.Now()
		s.Steps[stepIndex].ExecutedAt = &executedAt
		s.FailedSteps++

		// 如果失败次数超过阈值，设置整个Saga为失败
		if s.FailedSteps >= s.MaxRetryCount {
			s.setStatusLocked(StatusFailed)
		}

		s.UpdatedAt = time.Now()
		s.Version++
	}
}

// RetryStep 重试步骤
func (s *Saga) RetryStep(stepIndex int) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if stepIndex >= 0 && stepIndex < len(s.Steps) {
		if s.Steps[stepIndex].RetryCount < s.Steps[stepIndex].MaxRetries {
			s.Steps[stepIndex].RetryCount++
			s.RetryCount++
			s.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// IsCompleted 检查Saga是否完成
func (s *Saga) IsCompleted() bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return s.Status == StatusCompleted
}

// IsFailed 检查Saga是否失败
func (s *Saga) IsFailed() bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return s.Status == StatusFailed
}

// IsRunning 检查Saga是否正在运行
func (s *Saga) IsRunning() bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return s.Status == StatusRunning
}
