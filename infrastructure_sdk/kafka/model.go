package kafka

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

// NewBusinessEvent 创建一个新的业务事件
// 当 data 是 []byte 时直接作为 RawMessage（避免二次 json.Marshal 导致的 base64 编码）
func NewBusinessEvent(eventType, eventName, eventID string, data interface{}) (*BusinessEvent, error) {
	var rawData json.RawMessage
	switch v := data.(type) {
	case []byte:
		rawData = json.RawMessage(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		rawData = json.RawMessage(b)
	}

	return &BusinessEvent{
		CommonParams: CommonParams{
			EventType: eventType,
			EventName: eventName,
			EventID:   eventID,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		Data: rawData,
	}, nil
}
