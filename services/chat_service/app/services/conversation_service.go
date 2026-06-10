package services

import (
	"chat_service/app/database/pgsql/query"
	"chat_service/app/database/pgsql/model"
	"chat_service/app/database/pgsql"
	"context"
	"time"

	"gorm.io/gorm"
)

// ConversationService 会话服务
type ConversationService struct{}

// 单例
var conversationServiceInstance *ConversationService

// GetConversationService 获取会话服务实例
func GetConversationService() *ConversationService {
	if conversationServiceInstance == nil {
		conversationServiceInstance = &ConversationService{}
	}
	return conversationServiceInstance
}

// GetDB 获取数据库连接
func (s *ConversationService) GetDB() *gorm.DB {
	return pgsql.GetDBManager().GetDB()
}

// MarkMessageAsRead 标记消息为已读（更新最后阅读时间）
// userID: 用户ID
// conversationID: 会话ID (群ID 或 私聊会话ID)
func (s *ConversationService) MarkMessageAsRead(ctx context.Context, userID, conversationID string) error {
	db := s.GetDB()
	conv := query.Conversation

	// 查询当前会话记录
	record, err := conv.WithContext(ctx).Where(
		conv.UserID.Eq(userID),
		conv.ConversationID.Eq(conversationID),
	).First()

	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	now := time.Now()

	if err == gorm.ErrRecordNotFound {
		// 不存在，创建新记录
		newConv := &model.Conversation{
			UserID:           userID,
			ConversationID:   conversationID,
			LastReadTime:     now,
		}
		return db.Create(newConv).Error
	}

	// 存在，更新最后阅读时间
	record.LastReadTime = now
	return db.Save(record).Error
}

// MarkMessagesAsRead 批量标记消息为已读（更新最后阅读时间）
func (s *ConversationService) MarkMessagesAsRead(ctx context.Context, userID, conversationID string) error {
	return s.MarkMessageAsRead(ctx, userID, conversationID)
}

// GetLastReadTime 获取用户最后阅读时间
func (s *ConversationService) GetLastReadTime(ctx context.Context, userID, conversationID string) (time.Time, error) {
	conv := query.Conversation

	record, err := conv.WithContext(ctx).Where(
		conv.UserID.Eq(userID),
		conv.ConversationID.Eq(conversationID),
	).First()

	if err == gorm.ErrRecordNotFound {
		return time.Time{}, nil
	}

	if err != nil {
		return time.Time{}, err
	}

	return record.LastReadTime, nil
}

// GetConversation 获取用户会话信息
func (s *ConversationService) GetConversation(ctx context.Context, userID, conversationID string) (*model.Conversation, error) {
	conv := query.Conversation

	record, err := conv.WithContext(ctx).Where(
		conv.UserID.Eq(userID),
		conv.ConversationID.Eq(conversationID),
	).First()

	if err != nil {
		return nil, err
	}

	return record, nil
}

// UpdateLastReadTimeWhenOffline 用户离线时更新所有会话的最后阅读时间
func (s *ConversationService) UpdateLastReadTimeWhenOffline(ctx context.Context, userID string) error {
	db := s.GetDB()
	now := time.Now()

	// 更新该用户所有会话的最后阅读时间为当前时间
	return db.Model(&model.Conversation{}).
		Where("user_id = ?", userID).
		Update("last_read_time", now).Error
}

// GetUserConversationIDs 获取用户所有会话 ID 列表
func (s *ConversationService) GetUserConversationIDs(ctx context.Context, userID string) ([]string, error) {
	conv := query.Conversation

	records, err := conv.WithContext(ctx).Where(
		conv.UserID.Eq(userID),
	).Find()

	if err != nil {
		return nil, err
	}

	conversationIDs := make([]string, len(records))
	for i, record := range records {
		conversationIDs[i] = record.ConversationID
	}

	return conversationIDs, nil
}
