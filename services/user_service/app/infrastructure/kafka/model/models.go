package model

import (
	"encoding/json"
	"time"
)

// CommonParams 通用参数结构
type CommonParams struct {
	EventType     string `json:"event_type"`
	EventName     string `json:"event_name"`
	EventID       string `json:"event_id"`
	Timestamp     string `json:"timestamp"`
	ExecutionMode string `json:"execution_mode"` // "serial" or "parallel"
}

// BusinessEvent 通用业务事件结构
type BusinessEvent struct {
	CommonParams CommonParams    `json:"common_params"`
	Data         json.RawMessage `json:"data"`
}

// EventWrapper 是 user_service 内部产生的简单事件，可能需要被封装成 Saga 事件
type EventWrapper struct {
	EventType string          `json:"event_type"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

// KickUserData 定义踢人事件的具体数据结构
type KickUserData struct {
	UserID string `json:"user_id"`
	Reason string `json:"reason"`
}
