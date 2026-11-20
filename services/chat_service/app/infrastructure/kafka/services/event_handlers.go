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

// BroadcastAiReply 将 AI 完整回复推送给目标用户。
func BroadcastAiReply(reply *chatv1.AiReplyGenerated, ts int64) {
	if aiReplyBroadcast == nil {
		return
	}
	payload := map[string]interface{}{
		"type":            "ai_chat",
		"conversation_id": reply.ConversationId,
		"sender":          reply.SenderId,
		"content":         reply.Content,
		"message_id":      reply.MessageId,
		"reply_to_msg_id": reply.ReplyToMsgId,
		"metadata":        reply.Metadata,
		"time":            ts,
	}
	message, _ := json.Marshal(payload)
	aiReplyBroadcast(reply.TargetUserId, message)
}

// BroadcastAiReplyDelta 将 AI 流式分块实时推送给目标用户（不落库）。
func BroadcastAiReplyDelta(delta *chatv1.AiReplyDelta, ts int64) {
	if aiReplyBroadcast == nil {
		return
	}
	payload := map[string]interface{}{
		"type":            "ai_chat",
		"conversation_id": delta.ConversationId,
		"sender":          delta.SenderId,
		"content":         delta.Content,
		"message_id":      delta.MessageId,
		"reply_to_msg_id": delta.ReplyToMsgId,
		"kind":            delta.Kind,
		"seq":             delta.Seq,
		"time":            ts,
	}
	message, _ := json.Marshal(payload)
	aiReplyBroadcast(delta.TargetUserId, message)
}
