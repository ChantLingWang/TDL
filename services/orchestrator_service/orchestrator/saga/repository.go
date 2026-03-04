package saga

import (
	"context"
	"errors"
	"time"
)

// ErrVersionConflict 乐观锁版本冲突错误
// 当多个实例同时尝试更新同一个Saga时，只有一个会成功，其他的会返回此错误
var ErrVersionConflict = errors.New("saga version conflict: concurrent modification detected")

// ErrSagaNotFound Saga不存在错误
var ErrSagaNotFound = errors.New("saga not found")

// SagaRepository Saga持久化仓库接口
type SagaRepository interface {
	// Create 创建新的Saga
	// 新创建的Saga的Version会被设置为1
	Create(ctx context.Context, saga *Saga) error

	// Save 保存Saga（使用乐观锁更新）
	// 更新时会检查Version是否匹配，匹配则更新并递增Version
	// 如果Version不匹配，返回ErrVersionConflict错误
	Save(ctx context.Context, saga *Saga) error

	// Get 获取Saga
	// 如果Saga不存在，返回ErrSagaNotFound错误
	Get(ctx context.Context, sagaID string) (*Saga, error)

	// Delete 删除Saga
	Delete(ctx context.Context, sagaID string) error

	// Exists 检查Saga是否存在
	Exists(ctx context.Context, sagaID string) (bool, error)

	// AcquireLock 获取Saga的分布式锁
	// 如果锁未被持有或已过期，则获取锁并返回true
	// 如果锁已被其他实例持有且未过期，则返回false
	AcquireLock(ctx context.Context, sagaID string, instanceID string, leaseDuration time.Duration) (bool, error)

	// ReleaseLock 释放Saga的分布式锁
	// 只有持有锁的实例才能成功释放锁
	ReleaseLock(ctx context.Context, sagaID string, instanceID string) (bool, error)

	// RenewLock 续租Saga的分布式锁
	// 只有持有锁的实例才能成功续租
	RenewLock(ctx context.Context, sagaID string, instanceID string, leaseDuration time.Duration) (bool, error)
}
