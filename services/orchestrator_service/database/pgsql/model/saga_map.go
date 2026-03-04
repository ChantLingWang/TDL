package model

import (
	"time"

	"gorm.io/gorm"
)

// SagaMap Saga持久化模型
type SagaMap struct {
	ID            string `gorm:"primaryKey;size:64"`
	Status        string `gorm:"size:20;not null;index"`
	ExecutionMode string `gorm:"size:20;not null"`
	CurrentStep   int    `gorm:"default:-1"`
	Version       int    `gorm:"default:0"`
	Context       string `gorm:"type:text"` // JSON序列化的上下文数据
	Steps         string `gorm:"type:text"` // JSON序列化的步骤数据

	// 统计字段
	CompletedSteps int `gorm:"default:0"`
	FailedSteps    int `gorm:"default:0"`
	RetryCount     int `gorm:"default:0"`
	MaxRetryCount  int `gorm:"default:3"`

	// 时间戳
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime;index"`

	// 追踪字段
	CorrelationID string `gorm:"size:128"`
	TraceID       string `gorm:"size:128"`

	// 分布式锁字段
	LockedBy   *string    `gorm:"size:128;index"` // 持有锁的实例ID
	LockExpiry *time.Time `gorm:"index"`          // 锁过期时间
}

// TableName 指定表名
func (SagaMap) TableName() string {
	return "saga_map"
}

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&SagaMap{})
}
