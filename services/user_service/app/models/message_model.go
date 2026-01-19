package models

import "encoding/json"

// DistributedChatMessage 用于在分布式节点间传递聊天消息
type DistributedChatMessage struct {
	TargetUserID string          `json:"target_user_id"` // 消息接收者 ID
	Message      json.RawMessage `json:"message"`        // 原始消息内容（保持原样转发）
}

// BroadcastChatMessage 用于批量广播消息
type BroadcastChatMessage struct {
	TargetUserIDs []string        `json:"target_user_ids"` // 目标用户 ID 列表
	Message       json.RawMessage `json:"message"`         // 原始消息内容
}
