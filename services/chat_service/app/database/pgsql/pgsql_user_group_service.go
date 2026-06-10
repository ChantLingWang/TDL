package pgsql

import (
	"context"
	"fmt"

	"chat_service/app/database/pgsql/model"
	"chat_service/app/database/pgsql/query"

	"gorm.io/gorm"
)

// UserGroupService 用户组群服务结构体
type UserGroupService struct {
	dbManager *DBManager
}

// NewUserGroupService 创建新的用户组群服务实例
func NewUserGroupService(dbManager *DBManager) *UserGroupService {
	return &UserGroupService{
		dbManager: dbManager,
	}
}

// GetUserGroups 获取用户所属的所有组群
func (ugs *UserGroupService) GetUserGroups(userID string) ([]model.Group, error) {
	g := query.Group
	ug := query.UserGroup

	// 通过用户ID查询其所属的所有组群
	// Joins: JOIN user_groups ON user_groups.group_id = groups.group_id (注意这里假设 GroupID 对应 group_id)
	groups, err := g.WithContext(context.Background()).Join(ug, g.GroupID.EqCol(ug.GroupID)).
		Where(ug.UserID.Eq(userID)).
		Find()

	if err != nil {
		return nil, fmt.Errorf("failed to get user groups: %w", err)
	}

	// 转换 []*model.Group 为 []model.Group
	var result []model.Group
	for _, group := range groups {
		result = append(result, *group)
	}

	return result, nil
}

// AddUserToGroup 将用户添加到指定组群
func (ugs *UserGroupService) AddUserToGroup(userID, groupID string) error {
	u := query.User
	g := query.Group
	ug := query.UserGroup

	// 检查用户是否存在
	if _, err := u.WithContext(context.Background()).Where(u.UserID.Eq(userID)).First(); err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("user not found: %s", userID)
		}
		return err
	}

	// 检查组群是否存在
	if _, err := g.WithContext(context.Background()).Where(g.GroupID.Eq(groupID)).First(); err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("group not found: %s", groupID)
		}
		return err
	}

	// 创建用户组群关联
	userGroup := model.UserGroup{
		UserID:  userID,
		GroupID: groupID,
	}

	// 使用FirstOrCreate避免重复添加
	// 先查询是否存在
	_, err := ug.WithContext(context.Background()).Where(ug.UserID.Eq(userID), ug.GroupID.Eq(groupID)).First()
	if err == nil {
		return nil // 已存在
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	// 不存在则创建
	err = ug.WithContext(context.Background()).Create(&userGroup)
	if err != nil {
		return err
	}

	return nil
}

// RemoveUserFromGroup 将用户从指定组群移除
func (ugs *UserGroupService) RemoveUserFromGroup(userID, groupID string) error {
	ug := query.UserGroup
	info, err := ug.WithContext(context.Background()).Where(ug.UserID.Eq(userID), ug.GroupID.Eq(groupID)).Delete()
	if err != nil {
		return fmt.Errorf("failed to remove user from group: %w", err)
	}

	// 检查是否有记录被删除
	if info.RowsAffected == 0 {
		return fmt.Errorf("user-group relationship not found")
	}

	return nil
}

// CreateGroup 创建新的组群
func (ugs *UserGroupService) CreateGroup(groupID, groupName string) (*model.Group, error) {
	group := &model.Group{
		GroupID:   groupID,
		GroupName: groupName,
	}

	g := query.Group
	err := g.WithContext(context.Background()).Create(group)
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	return group, nil
}

// DeleteGroup 删除组群
func (ugs *UserGroupService) DeleteGroup(groupID string) error {
	ug := query.UserGroup
	g := query.Group

	// 先删除所有关联的用户组群关系
	if _, err := ug.WithContext(context.Background()).Where(ug.GroupID.Eq(groupID)).Delete(); err != nil {
		return fmt.Errorf("failed to delete user-group relationships: %w", err)
	}

	// 再删除组群本身
	info, err := g.WithContext(context.Background()).Where(g.GroupID.Eq(groupID)).Delete()
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	// 检查是否有记录被删除
	if info.RowsAffected == 0 {
		return fmt.Errorf("group not found")
	}

	return nil
}

// GetGroupByID 根据ID获取组群信息
func (ugs *UserGroupService) GetGroupByID(groupID string) (*model.Group, error) {
	g := query.Group
	group, err := g.WithContext(context.Background()).Where(g.GroupID.Eq(groupID)).First()
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("group not found: %s", groupID)
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}
	return group, nil
}

// GetGroupMembers 获取组群的所有成员ID
func (ugs *UserGroupService) GetGroupMembers(groupID string) ([]string, error) {
	ug := query.UserGroup
	userGroups, err := ug.WithContext(context.Background()).Where(ug.GroupID.Eq(groupID)).Find()
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}

	// 提取用户ID
	var memberIDs []string
	for _, item := range userGroups {
		memberIDs = append(memberIDs, item.UserID)
	}
	return memberIDs, nil
}

// GenerateGroupID 使用 PostgreSQL Sequence 生成递增的 group_id
func (ugs *UserGroupService) GenerateGroupID() (string, error) {
	// 直接获取下一个 Sequence 值（Sequence 已在 Initialize 时创建）
	var nextVal int64
	if err := ugs.dbManager.GetDB().Raw("SELECT nextval('group_id_seq')").Scan(&nextVal).Error; err != nil {
		return "", fmt.Errorf("failed to get next sequence value: %w", err)
	}

	// 转换为字符串格式 "G" + 序号（如 G1, G2, G3...）
	groupID := fmt.Sprintf("G%d", nextVal)
	return groupID, nil
}
