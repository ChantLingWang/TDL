package kafka

import (
	"time"
)

// DLQPayload 死信队列消息体
// 包含原始消息数据和失败原因
type DLQPayload struct {
	OriginalTopic string         `json:"original_topic"`
	OriginalEvent *BusinessEvent `json:"original_event"`
	Reason        string         `json:"reason"`
	FailedAt      time.Time      `json:"failed_at"`
	Service       string         `json:"service,omitempty"` // 产生死信的服务
}

// NewDLQPayload 创建死信负载
func NewDLQPayload(originalTopic string, event *BusinessEvent, reason string) *DLQPayload {
	return &DLQPayload{
		OriginalTopic: originalTopic,
		OriginalEvent: event,
		Reason:        reason,
		FailedAt:      time.Now(),
	}
}

// DLQEventType 死信队列事件类型
const DLQEventType = "sys.dlq.message"
