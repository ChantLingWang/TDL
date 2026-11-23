package services

import (
	"encoding/json"

	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
)

// aiReplyBroadcast 向目标用户的本地 WS 连接推送消息。
var aiReplyBroadcast func(targetUserID string, message []byte)

// RegisterAiReplyBroadcast 注册 AI 事件推送回调。
func RegisterAiReplyBroadcast(fn func(targetUserID string, message []byte)) {
	aiReplyBroadcast = fn
}

// BroadcastAiTurnStep 将 AI 的一步推送给目标用户。
// proto 里 payload 是 typed oneof，这里转成前端认识的 kind + JSON 对象。
func BroadcastAiTurnStep(step *chatv1.AiTurnStep, ts int64) {
	if aiReplyBroadcast == nil {
		return
	}

	kind, payload, ok := stepPayload(step)
	if !ok {
		return
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
		"kind":            kind,
		"payload":         payload,
		"time":            ts,
	}

	message, err := json.Marshal(body)
	if err != nil {
		return
	}
	aiReplyBroadcast(step.TargetUserId, message)
}

func stepPayload(step *chatv1.AiTurnStep) (string, interface{}, bool) {
	switch p := step.Payload.(type) {
	case *chatv1.AiTurnStep_Think:
		return "THINK", map[string]interface{}{
			"think": p.Think.GetThink(),
		}, true

	case *chatv1.AiTurnStep_ToolCall:
		calls := make([]map[string]interface{}, 0, len(p.ToolCall.GetCalls()))
		for _, call := range p.ToolCall.GetCalls() {
			var input interface{}
			if call.GetInput() != nil {
				input = call.GetInput().AsInterface()
			}
			calls = append(calls, map[string]interface{}{
				"id":    call.GetId(),
				"name":  call.GetName(),
				"input": input,
			})
		}
		return "TOOL_CALL", map[string]interface{}{"calls": calls}, true

	case *chatv1.AiTurnStep_ToolResult:
		results := make([]map[string]interface{}, 0, len(p.ToolResult.GetResults()))
		for _, result := range p.ToolResult.GetResults() {
			var payload interface{}
			if result.GetPayload() != nil {
				payload = result.GetPayload().AsInterface()
			}
			results = append(results, map[string]interface{}{
				"id":          result.GetId(),
				"name":        result.GetName(),
				"payload":     payload,
				"status":      toolStatusString(result.GetStatus()),
				"duration_ms": result.GetDurationMs(),
				"error":       result.GetError(),
			})
		}
		return "TOOL_RESULT", map[string]interface{}{"results": results}, true

	case *chatv1.AiTurnStep_Result:
		return "RESULT", map[string]interface{}{
			"result": p.Result.GetResult(),
			"status": resultStatusString(p.Result.GetStatus()),
			"reason": p.Result.GetReason(),
		}, true

	default:
		return "", nil, false
	}
}

func toolStatusString(status chatv1.ToolStatus) string {
	switch status {
	case chatv1.ToolStatus_TOOL_STATUS_COMPLETED:
		return "completed"
	case chatv1.ToolStatus_TOOL_STATUS_FAILED:
		return "failed"
	case chatv1.ToolStatus_TOOL_STATUS_TIMEOUT:
		return "timeout"
	default:
		return "unspecified"
	}
}

func resultStatusString(status chatv1.ResultStatus) string {
	switch status {
	case chatv1.ResultStatus_RESULT_STATUS_COMPLETED:
		return "completed"
	case chatv1.ResultStatus_RESULT_STATUS_CANCELED:
		return "canceled"
	case chatv1.ResultStatus_RESULT_STATUS_FAILED:
		return "failed"
	default:
		return "unspecified"
	}
}
