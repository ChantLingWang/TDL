package kafka

import (
	"context"
	"log"

	"chat_service/app/infrastructure/kafka/services"
	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
	commonv1 "github.com/chant/chant/gen/go/chant/common/v1"
	sdk_kafka "infrastructure_sdk/kafka"
)

// HandleProtoEnvelope gateway 只消费 AI 回复；用户消息由 gateway 自己生产。
func HandleProtoEnvelope(ctx context.Context, env *commonv1.EventEnvelope) error {
	switch env.EventType {
	case "chant.chat.v1.AiReplyGenerated":
		return handleAiReply(ctx, env)
	case "chant.chat.v1.AiReplyDelta":
		return handleAiReplyDelta(ctx, env)
	case "chant.chat.v1.MessageSent":
		// gateway 自己发出的用户消息，不需要回环消费。
		return nil
	default:
		log.Printf("忽略未知事件类型: %s", env.EventType)
		return nil
	}
}

// handleAiReplyDelta 处理 AI 流式分块 —— 仅实时 WS 转发，不落库。
func handleAiReplyDelta(ctx context.Context, env *commonv1.EventEnvelope) error {
	delta := new(chatv1.AiReplyDelta)
	if err := sdk_kafka.UnmarshalData(env, delta); err != nil {
		return err
	}
	services.BroadcastAiReplyDelta(delta, env.Timestamp)
	return nil
}

// handleAiReply 处理 AI 完整回复 —— 仅实时 WS 转发，不落库。
func handleAiReply(ctx context.Context, env *commonv1.EventEnvelope) error {
	reply := new(chatv1.AiReplyGenerated)
	if err := sdk_kafka.UnmarshalData(env, reply); err != nil {
		return err
	}
	services.BroadcastAiReply(reply, env.Timestamp)
	return nil
}
