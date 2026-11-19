package services

import (
	"encoding/json"
	"testing"

	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
)

// TestBroadcastAiReplyDeltaAI 验证 AI-only delta 的 WS 广播载荷。
func TestBroadcastAiReplyDeltaAI(t *testing.T) {
	var capturedTarget string
	var capturedPayload map[string]interface{}

	aiReplyBroadcast = func(targetUserID string, message []byte) {
		capturedTarget = targetUserID
		_ = json.Unmarshal(message, &capturedPayload)
	}
	defer func() { aiReplyBroadcast = nil }()

	delta := &chatv1.AiReplyDelta{
		SenderId:       "ai-assistant",
		TargetUserId:   "user-1",
		ConversationId: "conv-1",
		ReplyToMsgId:   "m-1",
		MessageId:      "ai-m-1",
		Seq:            3,
		Kind:           "thinking",
		Content:        "正在分析",
		TimestampMs:    1786000000000,
		Metadata:       map[string]string{"k": "v"},
	}

	BroadcastAiReplyDelta(delta, 1786000000001)

	if capturedTarget != "user-1" {
		t.Fatalf("广播目标错误: target=%s", capturedTarget)
	}
	if capturedPayload["type"] != "ai_chat" {
		t.Fatalf("type 应为 ai_chat, got %v", capturedPayload["type"])
	}
	if capturedPayload["conversation_id"] != "conv-1" {
		t.Fatalf("conversation_id 错误: %v", capturedPayload["conversation_id"])
	}
	if capturedPayload["kind"] != "thinking" || capturedPayload["seq"] != float64(3) {
		t.Fatalf("kind/seq 错误: %v", capturedPayload)
	}
	if capturedPayload["message_id"] != "ai-m-1" || capturedPayload["reply_to_msg_id"] != "m-1" {
		t.Fatalf("message_id/reply_to 错误: %v", capturedPayload)
	}
}
