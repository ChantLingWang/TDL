package services

import (
	"context"
	"log"
	"sync"

	"chat_service/app/api/models"
	"chat_service/app/config"
	chatconst "chat_service/app/const"
	"chat_service/app/infrastructure/kafka"

	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
	"github.com/google/uuid"
)

// roundRequest 一次待分配的发送请求。
type roundRequest struct {
	userInfo *UserInfo
	content  models.ChatMessageRequest
}

// RoundAllocator 用单个 goroutine 独占 per-conversation 的轮次计数，
// 并在同一个串行段里把 MessageSent 发到 Kafka，保证 round 顺序与 Kafka 顺序一致。
type RoundAllocator struct {
	requests chan roundRequest
	rounds   map[string]int64
}

var (
	allocator     *RoundAllocator
	allocatorOnce sync.Once
)

// GetRoundAllocator 获取全局分配器（懒启动一个 goroutine）。
func GetRoundAllocator() *RoundAllocator {
	allocatorOnce.Do(func() {
		allocator = &RoundAllocator{
			requests: make(chan roundRequest, 256),
			rounds:   make(map[string]int64),
		}
		go allocator.run()
	})
	return allocator
}

// Submit 提交一条待发送消息；只做校验，实际 round 分配在 run 里串行完成。
func (r *RoundAllocator) Submit(userInfo *UserInfo, content models.ChatMessageRequest) {
	if content.Text == "" {
		return
	}
	r.requests <- roundRequest{userInfo: userInfo, content: content}
}

func (r *RoundAllocator) run() {
	for req := range r.requests {
		r.publish(req)
	}
}

func (r *RoundAllocator) publish(req roundRequest) {
	conversationID := req.content.ConversationID
	if conversationID == "" {
		conversationID = uuid.NewString()
	}

	messageID := req.content.MessageID
	if messageID == "" {
		messageID = uuid.NewString()
	}

	messageType := req.content.MessageType
	if messageType == "" {
		messageType = chatconst.MessageTypeText
	}

	round := r.nextRound(conversationID)

	msg := &chatv1.MessageSent{
		SenderId:       req.userInfo.UserID,
		TargetUserId:   config.AIUserID,
		ConversationId: conversationID,
		Content:        req.content.Text,
		MessageId:      messageID,
		MessageType:    messageType,
		Round:          round,
	}

	if err := kafka.GetProducer().SendProtoEvent(
		context.Background(), "chant.chat.v1.MessageSent", conversationID, msg,
	); err != nil {
		log.Printf("发送 MessageSent 失败 user=%s conversation=%s round=%d err=%v",
			req.userInfo.UserID, conversationID, round, err)
	}
}

// nextRound 只由 run goroutine 调用，因此不需要锁。
func (r *RoundAllocator) nextRound(conversationID string) int64 {
	r.rounds[conversationID]++
	return r.rounds[conversationID]
}
