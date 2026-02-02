package kafka

import (
	"encoding/json"
	"time"
)

// CommonParams 通用参数结构
// 用于 Saga 编排和复杂业务事件
type CommonParams struct {
	EventType     string `json:"event_type"`
	EventName     string `json:"event_name"`
	EventID       string `json:"event_id"`
	Timestamp     string `json:"timestamp"`
	ExecutionMode string `json:"execution_mode"` // "serial" or "parallel"
}

// BusinessEvent 通用业务事件结构
// 这是系统内部服务间通信的标准格式
type BusinessEvent struct {
	CommonParams CommonParams    `json:"common_params"`
	Data         json.RawMessage `json:"data"`
}

// NewBusinessEvent 创建一个新的业务事件
func NewBusinessEvent(eventType, eventName, eventID string, data interface{}) (*BusinessEvent, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &BusinessEvent{
		CommonParams: CommonParams{
			EventType: eventType,
			EventName: eventName,
			EventID:   eventID,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		Data: dataBytes,
	}, nil
}
