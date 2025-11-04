package pgsql

import (
	"fmt"

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
func (ugs *UserGroupService) GetUserGroups(userID string) ([]Group, error) {
	var groups []Group
	
	// 通过用户ID查询其所属的所有组群
	err := ugs.dbManager.GetDB().
		Joins("JOIN user_groups ON user_groups.group_id = groups.id").
		Where("user_groups.user_id = ?", userID).
		Find(&groups).Error
	
	if err != nil {
		return nil, fmt.Errorf("failed to get user groups: %w", err)
	}
	
	return groups, nil
}

// AddUserToGroup 将用户添加到指定组群
func (ugs *UserGroupService) AddUserToGroup(userID, groupID string) error {
	// 检查用户和组群是否存在
	var user User
	if err := ugs.dbManager.GetDB().First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("user not found: %s", userID)
		}
		return fmt.Errorf("failed to find user: %w", err)
	}
	
	var group Group
	if err := ugs.dbManager.GetDB().First(&group, "id = ?", groupID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("group not found: %s", groupID)
		}
		return fmt.Errorf("failed to find group: %w", err)
	}
	
	// 创建用户组群关联
	userGroup := UserGroup{
		UserID:  userID,
		GroupID: groupID,
	}
	
	// 使用FirstOrCreate避免重复添加
	result := ugs.dbManager.GetDB().Where(UserGroup{UserID: userID, GroupID: groupID}).FirstOrCreate(&userGroup)
	if result.Error != nil {
		return fmt.Errorf("failed to add user to group: %w", result.Error)
	}
	
	return nil
}

// RemoveUserFromGroup 将用户从指定组群移除
func (ugs *UserGroupService) RemoveUserFromGroup(userID, groupID string) error {
	result := ugs.dbManager.GetDB().Where("user_id = ? AND group_id = ?", userID, groupID).Delete(&UserGroup{})
	if result.Error != nil {
		return fmt.Errorf("failed to remove user from group: %w", result.Error)
	}
	
	// 检查是否有记录被删除
	if result.RowsAffected == 0 {
		return fmt.Errorf("user-group relationship not found")
	}
	
	return nil
}


// CreateGroup 创建新的组群
func (ugs *UserGroupService) CreateGroup(groupID, groupName string) (*Group, error) {
	group := &Group{
		GroupID: groupID,
		GroupName: groupName,
	}
	
	result := ugs.dbManager.GetDB().Create(group)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to create group: %w", result.Error)
	}
	
	return group, nil
}


// DeleteGroup 删除组群
func (ugs *UserGroupService) DeleteGroup(groupID string) error {
	// 先删除所有关联的用户组群关系
	if err := ugs.dbManager.GetDB().Where("group_id = ?", groupID).Delete(&UserGroup{}).Error; err != nil {
		return fmt.Errorf("failed to delete user-group relationships: %w", err)
	}
	
	// 再删除组群本身
	result := ugs.dbManager.GetDB().Delete(&Group{}, "id = ?", groupID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete group: %w", result.Error)
	}
	
	// 检查是否有记录被删除
	if result.RowsAffected == 0 {
		return fmt.Errorf("group not found")
	}
	
	return nil
}

// GetGroupByID 根据ID获取组群信息
func (ugs *UserGroupService) GetGroupByID(groupID string) (*Group, error) {
	var group Group
	result := ugs.dbManager.GetDB().First(&group, "id = ?", groupID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("group not found: %s", groupID)
		}
		return nil, fmt.Errorf("failed to get group: %w", result.Error)
	}
	return &group, nil
}