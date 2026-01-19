package models

import (
	"time"
)

// BaseEvent 基础事件模型
type BaseEvent struct {
	EventID      string    `json:"event_id"`
	Timestamp    time.Time `json:"timestamp"`
	EventProducer string   `json:"event_producer"`
}

// UserRegisteredEvent 用户注册事件
type UserRegisteredEvent struct {
	BaseEvent
	EventType string                 `json:"event_type"`
	UserID    string                 `json:"user_id"`
	Username  string                 `json:"username"`
	Email     string                 `json:"email"`
	CreatedAt string                 `json:"created_at"`
	Data      map[string]interface{} `json:"data,omitempty"`
}