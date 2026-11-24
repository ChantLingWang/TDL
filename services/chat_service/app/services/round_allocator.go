package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"chat_service/app/api/models"
	"chat_service/app/config"
	chatconst "chat_service/app/const"
	"chat_service/app/infrastructure/kafka"

	chatv1 "github.com/chant/chant/gen/go/chant/chat/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	maxParts      = 8
	maxPartBytes  = 5 << 20  // 5MB / part
	maxTotalBytes = 10 << 20 // 10MB / message
	maxMetaKeys   = 32
	maxMetaBytes  = 4 << 10 // 4KB
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

// Submit 提交一条待发送消息；只做非空校验，实际 round 分配在 run 里串行完成。
func (r *RoundAllocator) Submit(userInfo *UserInfo, content models.ChatMessageRequest) {
	if strings.TrimSpace(content.Text) == "" && len(content.Parts) == 0 {
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
	parts, err := buildParts(req.content)
	if err != nil {
		log.Printf("消息内容非法 user=%s conversation=%s err=%v",
			req.userInfo.UserID, req.content.ConversationID, err)
		return
	}

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
		MessageId:      messageID,
		MessageType:    messageType,
		Round:          round,
		Parts:          parts,
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

// buildParts 校验并转换前端 parts；兼容旧客户端的 text。
func buildParts(content models.ChatMessageRequest) ([]*chatv1.ContentPart, error) {
	rawParts := content.Parts
	if len(rawParts) == 0 {
		if strings.TrimSpace(content.Text) == "" {
			return nil, errors.New("消息内容为空")
		}
		rawParts = []models.ChatPart{{Type: "text", Text: content.Text}}
	}

	if len(rawParts) > maxParts {
		return nil, fmt.Errorf("parts 最多 %d 个", maxParts)
	}

	parts := make([]*chatv1.ContentPart, 0, len(rawParts))
	textCount := 0
	totalBytes := 0

	for index, raw := range rawParts {
		partType := strings.ToLower(strings.TrimSpace(raw.Type))
		if partType == "" {
			partType = "text"
		}

		part := &chatv1.ContentPart{
			Type:     partType,
			Text:     raw.Text,
			Data:     raw.Data,
			MimeType: raw.MimeType,
		}

		switch partType {
		case "text":
			textCount++
			if textCount > 1 {
				return nil, errors.New("最多一个文本 part")
			}
			if strings.TrimSpace(raw.Text) == "" {
				return nil, errors.New("文本 part 不能为空")
			}
			if len(raw.Data) > 0 {
				return nil, errors.New("文本 part 不允许 data")
			}
			if index != len(rawParts)-1 {
				return nil, errors.New("文本 part 必须放在最后")
			}

		case "image", "audio", "video":
			if len(raw.Data) == 0 {
				return nil, fmt.Errorf("%s part 缺少 data", partType)
			}
			if strings.TrimSpace(raw.MimeType) == "" {
				return nil, fmt.Errorf("%s part 缺少 mime_type", partType)
			}
			if strings.TrimSpace(raw.Text) != "" {
				return nil, fmt.Errorf("%s part 不允许 text", partType)
			}
			if len(raw.Data) > maxPartBytes {
				return nil, fmt.Errorf("%s part 超过 %d bytes", partType, maxPartBytes)
			}
			totalBytes += len(raw.Data)
			if totalBytes > maxTotalBytes {
				return nil, fmt.Errorf("媒体总大小超过 %d bytes", maxTotalBytes)
			}

		default:
			return nil, fmt.Errorf("不支持的 part type: %s", partType)
		}

		if len(raw.Metadata) > 0 {
			if len(raw.Metadata) > maxMetaKeys {
				return nil, fmt.Errorf("metadata 最多 %d 个 key", maxMetaKeys)
			}
			rawMeta, err := json.Marshal(raw.Metadata)
			if err != nil {
				return nil, fmt.Errorf("metadata 不是合法 JSON: %w", err)
			}
			if len(rawMeta) > maxMetaBytes {
				return nil, fmt.Errorf("metadata 超过 %d bytes", maxMetaBytes)
			}
			meta, err := structpb.NewStruct(raw.Metadata)
			if err != nil {
				return nil, fmt.Errorf("metadata 转换失败: %w", err)
			}
			part.Metadata = meta
		}

		parts = append(parts, part)
	}

	return parts, nil
}
