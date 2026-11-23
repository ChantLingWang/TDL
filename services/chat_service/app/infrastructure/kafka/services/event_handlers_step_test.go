package services

import (
	"encoding/json"
	"testing"

	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
)

// TestBroadcastAiTurnStep 验证 step 事件的 WS 载荷与 payload 嵌套。
func TestBroadcastAiTurnStep(t *testing.T) {
	var capturedTarget string
	var capturedPayload map[string]interface{}

	aiReplyBroadcast = func(targetUserID string, message []byte) {
		capturedTarget = targetUserID
		_ = json.Unmarshal(message, &capturedPayload)
	}
	defer func() { aiReplyBroadcast = nil }()

	step := &chatv1.AiTurnStep{
		SenderId:       "ai-assistant",
		TargetUserId:   "user-1",
		ConversationId: "conv-1",
		ReplyToMsgId:   "m-1",
		MessageId:      "ai-m-1",
		Round:          2,
		Step:           3,
		Attempt:        0,
		Kind:           "THINK",
		Payload:        `{"think":"正在分析"}`,
	}

	BroadcastAiTurnStep(step, 1786000000001)

	if capturedTarget != "user-1" {
		t.Fatalf("广播目标错误: target=%s", capturedTarget)
	}
	if capturedPayload["type"] != "ai_step" {
		t.Fatalf("type 应为 ai_step, got %v", capturedPayload["type"])
	}
	if capturedPayload["kind"] != "THINK" {
		t.Fatalf("kind 错误: %v", capturedPayload["kind"])
	}
	if capturedPayload["round"] != float64(2) || capturedPayload["step"] != float64(3) {
		t.Fatalf("round/step 错误: %v", capturedPayload)
	}
	payload, ok := capturedPayload["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload 应为嵌套对象: %T", capturedPayload["payload"])
	}
	if payload["think"] != "正在分析" {
		t.Fatalf("payload.think 错误: %v", payload["think"])
	}
}
