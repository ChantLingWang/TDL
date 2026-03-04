package pgsql

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"orchestrator_service/database/pgsql/model"
	"orchestrator_service/database/pgsql/query"
	"orchestrator_service/orchestrator/saga"
)

// PgsqlSagaRepository PostgreSQL实现的Saga仓库
type PgsqlSagaRepository struct{}

// NewPgsqlSagaRepository 创建PostgreSQL Saga仓库
func NewPgsqlSagaRepository() *PgsqlSagaRepository {
	return &PgsqlSagaRepository{}
}

// Create 创建新的Saga，新创建的Saga的Version会被设置为1
func (r *PgsqlSagaRepository) Create(ctx context.Context, s *saga.Saga) error {
	// 初始化版本号为1
	s.Version = 1

	sagaMap, err := sagaToModel(s)
	if err != nil {
		return fmt.Errorf("failed to convert saga to model: %w", err)
	}

	// 使用 Create 方法创建新记录
	return query.SagaMap.WithContext(ctx).Create(sagaMap)
}

// Save 保存Saga
// 如果Version不匹配，返回ErrVersionConflict错误
func (r *PgsqlSagaRepository) Save(ctx context.Context, s *saga.Saga) error {
	// 保存当前版本号用于乐观锁检查
	currentVersion := s.Version

	// 递增版本号
	s.Version = currentVersion + 1
	s.UpdatedAt = time.Now()

	sagaMap, err := sagaToModel(s)
	if err != nil {
		// 回滚版本号
		s.Version = currentVersion
		return fmt.Errorf("failed to convert saga to model: %w", err)
	}

	// 使用乐观锁更新：WHERE id = ? AND version = ?
	result, err := query.SagaMap.WithContext(ctx).
		Where(query.SagaMap.ID.Eq(s.ID)).
		Where(query.SagaMap.Version.Eq(currentVersion)).
		Updates(sagaMap)

	if err != nil {
		// 回滚版本号
		s.Version = currentVersion
		return fmt.Errorf("failed to update saga: %w", err)
	}

	// 检查是否有记录被更新
	if result.RowsAffected == 0 {
		// 回滚版本号
		s.Version = currentVersion
		// 没有记录被更新，说明版本冲突或记录不存在
		exists, _ := r.Exists(ctx, s.ID)
		if exists {
			return saga.ErrVersionConflict
		}
		return saga.ErrSagaNotFound
	}

	return nil
}

// Get 获取Saga
// 如果Saga不存在，返回ErrSagaNotFound错误
func (r *PgsqlSagaRepository) Get(ctx context.Context, sagaID string) (*saga.Saga, error) {
	sagaMap, err := query.SagaMap.WithContext(ctx).Where(query.SagaMap.ID.Eq(sagaID)).First()
	if err != nil {
		return nil, saga.ErrSagaNotFound
	}

	return modelToSaga(sagaMap)
}

// Delete 删除Saga
func (r *PgsqlSagaRepository) Delete(ctx context.Context, sagaID string) error {
	_, err := query.SagaMap.WithContext(ctx).Where(query.SagaMap.ID.Eq(sagaID)).Delete()
	return err
}

// Exists 检查Saga是否存在
func (r *PgsqlSagaRepository) Exists(ctx context.Context, sagaID string) (bool, error) {
	count, err := query.SagaMap.WithContext(ctx).Where(query.SagaMap.ID.Eq(sagaID)).Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// sagaToModel 将Saga转换为数据库模型
func sagaToModel(s *saga.Saga) (*model.SagaMap, error) {
	// 序列化Steps
	stepsJSON, err := json.Marshal(s.Steps)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal steps: %w", err)
	}

	// 序列化Context
	contextJSON, err := json.Marshal(s.Context)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal context: %w", err)
	}

	// 获取执行模式
	executionMode := ""
	if mode, ok := s.Context["execution_mode"].(string); ok {
		executionMode = mode
	}

	return &model.SagaMap{
		ID:             s.ID,
		Status:         string(s.Status),
		ExecutionMode:  executionMode,
		CurrentStep:    s.CurrentStep,
		Version:        s.Version,
		Context:        string(contextJSON),
		Steps:          string(stepsJSON),
		CompletedSteps: s.CompletedSteps,
		FailedSteps:    s.FailedSteps,
		RetryCount:     s.RetryCount,
		MaxRetryCount:  s.MaxRetryCount,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
		CorrelationID:  s.CorrelationID,
		TraceID:        s.TraceID,
		LockedBy:       s.LockedBy,
		LockExpiry:     s.LockExpiry,
	}, nil
}

// modelToSaga 将数据库模型转换为Saga
func modelToSaga(m *model.SagaMap) (*saga.Saga, error) {
	// 反序列化Steps
	var steps []saga.SagaStep
	if err := json.Unmarshal([]byte(m.Steps), &steps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal steps: %w", err)
	}

	// 反序列化Context
	var context map[string]any
	if m.Context != "" {
		if err := json.Unmarshal([]byte(m.Context), &context); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context: %w", err)
		}
	}
	if context == nil {
		context = make(map[string]any)
	}

	return &saga.Saga{
		ID:             m.ID,
		Status:         saga.SagaStatus(m.Status),
		Steps:          steps,
		CurrentStep:    m.CurrentStep,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
		Context:        context,
		Version:        m.Version,
		CorrelationID:  m.CorrelationID,
		TraceID:        m.TraceID,
		CompletedSteps: m.CompletedSteps,
		FailedSteps:    m.FailedSteps,
		RetryCount:     m.RetryCount,
		MaxRetryCount:  m.MaxRetryCount,
		LockedBy:       m.LockedBy,
		LockExpiry:     m.LockExpiry,
		Mu:             sync.Mutex{},
	}, nil
}

// FindByStatus 按状态查询Saga列表（用于服务恢复）
func (r *PgsqlSagaRepository) FindByStatus(ctx context.Context, status saga.SagaStatus) ([]*saga.Saga, error) {
	sagaMaps, err := query.SagaMap.WithContext(ctx).Where(query.SagaMap.Status.Eq(string(status))).Find()
	if err != nil {
		return nil, err
	}

	sagas := make([]*saga.Saga, 0, len(sagaMaps))
	for _, m := range sagaMaps {
		s, err := modelToSaga(m)
		if err != nil {
			return nil, err
		}
		sagas = append(sagas, s)
	}
	return sagas, nil
}

// AcquireLock 尝试获取Saga的分布式锁
// 如果锁未被持有或已过期，则获取锁成功并返回true
// 否则返回false
func (r *PgsqlSagaRepository) AcquireLock(ctx context.Context, sagaID string, instanceID string, leaseDuration time.Duration) (bool, error) {
	now := time.Now()
	expiry := now.Add(leaseDuration)

	result, err := query.SagaMap.WithContext(ctx).
		Where(query.SagaMap.ID.Eq(sagaID)).
		Where(query.SagaMap.LockedBy.IsNull()).Or(query.SagaMap.LockExpiry.Lt(now)).
		Updates(map[string]interface{}{
			"locked_by":   &instanceID,
			"lock_expiry": &expiry,
			"updated_at":  now,
		})

	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	return result.RowsAffected > 0, nil
}

// ReleaseLock 释放Saga的分布式锁
// 只有持有锁的实例才能成功释放锁
func (r *PgsqlSagaRepository) ReleaseLock(ctx context.Context, sagaID string, instanceID string) (bool, error) {
	now := time.Now()

	result, err := query.SagaMap.WithContext(ctx).
		Where(query.SagaMap.ID.Eq(sagaID)).
		Where(query.SagaMap.LockedBy.Eq(instanceID)).
		Updates(map[string]interface{}{
			"locked_by":   nil,
			"lock_expiry": nil,
			"updated_at":  now,
		})

	if err != nil {
		return false, fmt.Errorf("failed to release lock: %w", err)
	}
	return result.RowsAffected > 0, nil
}

// RenewLock 续租Saga的分布式锁
// 只有持有锁的实例才能成功续租
func (r *PgsqlSagaRepository) RenewLock(ctx context.Context, sagaID string, instanceID string, leaseDuration time.Duration) (bool, error) {
	now := time.Now()
	expiry := now.Add(leaseDuration)

	result, err := query.SagaMap.WithContext(ctx).
		Where(query.SagaMap.ID.Eq(sagaID)).
		Where(query.SagaMap.LockedBy.Eq(instanceID)).
		Where(query.SagaMap.LockExpiry.Gt(now)).
		Updates(map[string]interface{}{
			"lock_expiry": &expiry,
			"updated_at":  now,
		})

	if err != nil {
		return false, fmt.Errorf("failed to renew lock: %w", err)
	}
	return result.RowsAffected > 0, nil
}

// FindTimedOut 查询超时的Saga（用于超时检查）
func (r *PgsqlSagaRepository) FindTimedOut(ctx context.Context, timeout time.Duration) ([]*saga.Saga, error) {
	threshold := time.Now().Add(-timeout)
	sagaMaps, err := query.SagaMap.WithContext(ctx).
		Where(query.SagaMap.Status.Eq(string(saga.StatusRunning))).
		Where(query.SagaMap.UpdatedAt.Lt(threshold)).
		Find()
	if err != nil {
		return nil, err
	}

	sagas := make([]*saga.Saga, 0, len(sagaMaps))
	for _, m := range sagaMaps {
		s, err := modelToSaga(m)
		if err != nil {
			return nil, err
		}
		sagas = append(sagas, s)
	}
	return sagas, nil
}
