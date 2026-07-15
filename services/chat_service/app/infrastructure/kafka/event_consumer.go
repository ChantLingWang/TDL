package kafka

import (
	"context"
	"log"
	"time"

	"chat_service/app/database/mongodb"
	"chat_service/app/infrastructure/kafka/services"
	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
	commonv1 "github.com/chant/chant/gen/go/chant/common/v1"
	sdk_kafka "infrastructure_sdk/kafka"
)

func HandleProtoEnvelope(ctx context.Context, env *commonv1.EventEnvelope) error {
	switch env.EventType {
	case "chant.chat.v1.MessageSent":
		return handleMessageSent(ctx, env)
	case "chant.chat.v1.AiReplyGenerated":
		return handleAiReply(ctx, env)
	default:
		log.Printf("忽略未知事件类型: %s", env.EventType)
		return nil
	}
}

func handleMessageSent(ctx context.Context, env *commonv1.EventEnvelope) error {
	msg := new(chatv1.MessageSent)
	if err := sdk_kafka.UnmarshalData(env, msg); err != nil {
		return err
	}
	switch msg.ConversationType {
	case "group", "ai", "ai-research":
		return services.HandleGroupChatMessageEvent(ctx, env)
	case "private":
		return services.HandlePrivateChatMessageEvent(ctx, env)
	default:
		log.Printf("未知会话类型: %s", msg.ConversationType)
		return nil
	}
}

// handleAiReply 处理 AI 回复 —— 写库 + 委托 services 做 WS 推送。
func handleAiReply(ctx context.Context, env *commonv1.EventEnvelope) error {
	reply := new(chatv1.AiReplyGenerated)
	if err := sdk_kafka.UnmarshalData(env, reply); err != nil {
		return err
	}

	msg := &mongodb.Message{
		SenderID:    reply.SenderId,
		Timestamp:   time.Now(),
		TimestampMs: reply.TimestampMs,
		Content:     reply.Content,
		Metadata:    reply.Metadata,
		MessageID:   reply.MessageId,
		MessageType: "text",
		IsActive:    true,
	}

	if reply.GroupId != "" {
		msg.GroupID = reply.GroupId
		msg.ConversationID = reply.GroupId
		_ = mongodb.SaveMessage("ai", reply.SenderId, reply.GroupId, msg)
	} else {
		msg.PrivateID = reply.TargetUserId
		_ = mongodb.SaveMessage("private", reply.SenderId, reply.TargetUserId, msg)
	}

	// WS 推送 — 委托 services 包处理（有 GetWSHub 可用）
	services.BroadcastAiReply(reply, env.Timestamp)

	log.Printf("AI 回复已入库: target=%s", reply.TargetUserId)
	return nil
}
