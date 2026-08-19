package services

import (
	"encoding/json"
	"testing"

	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
)

// TestBroadcastAiReplyDeltaGroup 验证群聊 delta 的 WS 广播载荷。
func TestBroadcastAiReplyDeltaGroup(t *testing.T) {
	var capturedTarget string
	var capturedGroup string
	var capturedPayload map[string]interface{}

	// 注册一个假广播回调，捕获推送目标与 JSON
	aiReplyBroadcast = func(targetUserID, groupID string, message []byte) {
		capturedTarget = targetUserID
		capturedGroup = groupID
		_ = json.Unmarshal(message, &capturedPayload)
	}
	defer func() { aiReplyBroadcast = nil }()

	delta := &chatv1.AiReplyDelta{
		SenderId:      "ai-assistant",
		TargetUserId:  "user-1",
		GroupId:       "g-1",
		ReplyToMsgId:  "m-1",
		MessageId:     "ai-m-1",
		Seq:           3,
		Kind:          "thinking",
		Content:       "正在分析",
		TimestampMs:   1786000000000,
		Metadata:      map[string]string{"k": "v"},
	}

	BroadcastAiReplyDelta(delta, 1786000000001)

	if capturedTarget != "user-1" || capturedGroup != "g-1" {
		t.Fatalf("广播目标错误: target=%s group=%s", capturedTarget, capturedGroup)
	}
	if capturedPayload["type"] != "group_chat" {
		t.Fatalf("type 应为 group_chat, got %v", capturedPayload["type"])
	}
	if capturedPayload["kind"] != "thinking" || capturedPayload["seq"] != float64(3) {
		t.Fatalf("kind/seq 错误: %v", capturedPayload)
	}
	if capturedPayload["message_id"] != "ai-m-1" || capturedPayload["reply_to_msg_id"] != "m-1" {
		t.Fatalf("message_id/reply_to 错误: %v", capturedPayload)
	}
	if capturedPayload["content"] != "正在分析" {
		t.Fatalf("content 错误: %v", capturedPayload)
	}
}

// TestBroadcastAiReplyDeltaPrivate 验证私聊 delta 的 conversation_id 计算。
func TestBroadcastAiReplyDeltaPrivate(t *testing.T) {
	var capturedTarget string
	var capturedGroup string
	var capturedPayload map[string]interface{}

	aiReplyBroadcast = func(targetUserID, groupID string, message []byte) {
		capturedTarget = targetUserID
		capturedGroup = groupID
		_ = json.Unmarshal(message, &capturedPayload)
	}
	defer func() { aiReplyBroadcast = nil }()

	delta := &chatv1.AiReplyDelta{
		SenderId:      "ai-assistant",
		TargetUserId:  "user-2",
		ReplyToMsgId:  "m-2",
		MessageId:     "ai-m-2",
		Seq:           0,
		Kind:          "content",
		Content:       "答案",
	}

	BroadcastAiReplyDelta(delta, 1786000000001)

	if capturedTarget != "user-2" || capturedGroup != "" {
		t.Fatalf("私聊广播目标错误: target=%s group=%s", capturedTarget, capturedGroup)
	}
	if capturedPayload["type"] != "private_chat" {
		t.Fatalf("type 应为 private_chat, got %v", capturedPayload["type"])
	}
	if capturedPayload["conversation_id"] == "" {
		t.Fatal("私聊 conversation_id 不应为空")
	}
}
