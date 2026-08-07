package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
	commonv1 "github.com/chant/chant/gen/go/chant/common/v1"
	sdk_kafka "infrastructure_sdk/kafka"
)

var (
	// 群消息本地广播回调
	groupMessageLocalBroadcast func(groupID string, message []byte)

	// 私聊消息本地广播回调
	privateMessageLocalBroadcast func(targetUserID string, message []byte)
)

// RegisterGroupMessageLocalBroadcast 注册群消息本地广播函数
func RegisterGroupMessageLocalBroadcast(fn func(groupID string, message []byte)) {
	groupMessageLocalBroadcast = fn
}

// RegisterPrivateMessageLocalBroadcast 注册私聊消息本地广播函数
func RegisterPrivateMessageLocalBroadcast(fn func(targetUserID string, message []byte)) {
	privateMessageLocalBroadcast = fn
}

// HandleGroupChatMessageEvent 处理群聊消息 —— 从 proto envelope 解析
func HandleGroupChatMessageEvent(ctx context.Context, env *commonv1.EventEnvelope) error {
	msg := new(chatv1.MessageSent)
	if err := sdk_kafka.UnmarshalData(env, msg); err != nil {
		return fmt.Errorf("解析 MessageSent: %w", err)
	}

	log.Printf("收到群消息: group_id=%s, sender=%s, content=%s",
		msg.GroupId, msg.SenderId, msg.Content)

	responseMsg := map[string]interface{}{
		"type":            "group_chat",
		"conversation_id": msg.ConversationType,
		"group_id":        msg.GroupId,
		"sender":          msg.SenderId,
		"content":         msg.Content,
		"time":            env.Timestamp,
	}
	msgBytes, _ := json.Marshal(responseMsg)

	if groupMessageLocalBroadcast != nil {
		groupMessageLocalBroadcast(msg.GroupId, msgBytes)
		log.Printf("群消息已推送给本地在线用户: group_id=%s", msg.GroupId)
	} else {
		log.Printf("警告：未注册群消息本地广播函数")
	}
	return nil
}

// HandlePrivateChatMessageEvent 处理私聊消息 —— 从 proto envelope 解析
func HandlePrivateChatMessageEvent(ctx context.Context, env *commonv1.EventEnvelope) error {
	msg := new(chatv1.MessageSent)
	if err := sdk_kafka.UnmarshalData(env, msg); err != nil {
		return fmt.Errorf("解析 MessageSent: %w", err)
	}

	log.Printf("收到私聊消息: from=%s, to=%s, content=%s",
		msg.SenderId, msg.TargetUserId, msg.Content)

	responseMsg := map[string]interface{}{
		"type":            "private_chat",
		"conversation_id": msg.TargetUserId,
		"sender":          msg.SenderId,
		"content":         msg.Content,
		"time":            env.Timestamp,
	}
	msgBytes, _ := json.Marshal(responseMsg)

	if privateMessageLocalBroadcast != nil {
		privateMessageLocalBroadcast(msg.TargetUserId, msgBytes)
		log.Printf("私聊消息已推送给本地在线用户: to=%s", msg.TargetUserId)
	} else {
		log.Printf("警告：未注册私聊消息本地广播函数")
	}
	return nil
}

// BroadcastAiReply 将 AI 回复推送给目标用户。
func BroadcastAiReply(reply *chatv1.AiReplyGenerated, ts int64) {
	if aiReplyBroadcast == nil {
		return
	}
	payload := map[string]interface{}{
		"sender":  reply.SenderId,
		"content": reply.Content,
		"time":    ts,
	}
	if reply.GroupId != "" {
		payload["type"] = "group_chat"
		payload["group_id"] = reply.GroupId
	} else {
		payload["type"] = "private_chat"
	}
	responseMsg, _ := json.Marshal(payload)
	aiReplyBroadcast(reply.TargetUserId, reply.GroupId, responseMsg)
}

// RegisterAiReplyBroadcast 注册 AI 回复推送回调。
var aiReplyBroadcast func(targetUserID, groupID string, message []byte)

func RegisterAiReplyBroadcast(fn func(targetUserID, groupID string, message []byte)) {
	aiReplyBroadcast = fn
}
