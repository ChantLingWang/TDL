package consumer_model

import (
	"encoding/json"
)

// CommonParams 公共参数结构
type CommonParams struct {
	EventType     string `json:"event_type"`
	EventName     string `json:"event_name"`
	EventID       string `json:"event_id"`
	Timestamp     string `json:"timestamp"`
	ExecutionMode string `json:"execution_mode"`
}

// BusinessEvent 通用业务事件结构
type BusinessEvent struct {
	CommonParams CommonParams    `json:"common_params"`
	Data         json.RawMessage `json:"data"`
}
