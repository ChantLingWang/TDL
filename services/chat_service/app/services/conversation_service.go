package services

import (
	"fmt"
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

// ListConversations 列出用户的 AI 会话（群名以 ai_ 开头的群）
func (s *ConversationService) ListConversations(ctx context.Context, userID, convType string) ([]map[string]interface{}, error) {
	// 查 user_groups 联表 groups，过滤 ai_ 前缀
	var results []struct {
		GroupID   string
		GroupName string
	}
	db := s.GetDB()
	db.Table("user_groups").
		Select("user_groups.group_id, groups.group_name").
		Joins("JOIN groups ON groups.group_id = user_groups.group_id").
		Where("user_groups.user_id = ? AND groups.group_id LIKE ?", userID, "ai_%").
		Order("groups.create_time DESC").
		Scan(&results)

	list := make([]map[string]interface{}, len(results))
	for i, r := range results {
		list[i] = map[string]interface{}{
			"group_id": r.GroupID,
			"title":    r.GroupName,
		}
	}
	return list, nil
}

// CreateAIConversation 创建新的 AI 会话（一群一会话）
func (s *ConversationService) CreateAIConversation(ctx context.Context, userID, title string) (map[string]interface{}, error) {
	groupID := fmt.Sprintf("ai_%d", time.Now().UnixNano())
	now := time.Now()

	group := &model.Group{
		GroupID:        groupID,
		GroupName:      title,
		CreateByUserID: userID,
		CreateTime:     now,
	}
	if err := s.GetDB().Create(group).Error; err != nil {
		return nil, err
	}
	pgsql.NewUserGroupService(pgsql.GetDBManager()).AddUserToGroup(userID, groupID)

	return map[string]interface{}{
		"group_id": groupID,
		"title":    title,
	}, nil
}
