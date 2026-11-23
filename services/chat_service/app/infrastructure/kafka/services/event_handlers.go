package services

import (
	"encoding/json"

	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
)

// aiReplyBroadcast 向目标用户的本地 WS 连接推送消息。
var aiReplyBroadcast func(targetUserID string, message []byte)

// RegisterAiReplyBroadcast 注册 AI 回复推送回调。
func RegisterAiReplyBroadcast(fn func(targetUserID string, message []byte)) {
	aiReplyBroadcast = fn
}

// BroadcastAiTurnStep 将 AI 的一步推送给目标用户，payload 作为嵌套 JSON 发送。
func BroadcastAiTurnStep(step *chatv1.AiTurnStep, ts int64) {
	if aiReplyBroadcast == nil {
		return
	}

	// payload 本身是 JSON 字符串；能解析就作为对象嵌套，否则原样当字符串发。
	var payload interface{} = step.Payload
	if json.Valid([]byte(step.Payload)) {
		payload = json.RawMessage(step.Payload)
	}

	body := map[string]interface{}{
		"type":            "ai_step",
		"conversation_id": step.ConversationId,
		"sender":          step.SenderId,
		"reply_to_msg_id": step.ReplyToMsgId,
		"message_id":      step.MessageId,
		"round":           step.Round,
		"step":            step.Step,
		"attempt":         step.Attempt,
		"kind":            step.Kind,
		"payload":         payload,
		"time":            ts,
	}

	message, err := json.Marshal(body)
	if err != nil {
		return
	}
	aiReplyBroadcast(step.TargetUserId, message)
}
