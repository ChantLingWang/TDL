package pgsql

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

 type UserService struct {
	dbManager *DBManager
 }


// NewUserService 创建新的用户服务实例
func NewUserService(dbManager *DBManager) *UserService {
	return &UserService{
		dbManager: dbManager,
	}
}


func (service *UserService) GetUserByID(ctx context.Context, userID string) (*User, error) {
	db := service.dbManager.GetDB()
	
	// 从数据库中查询用户
	var user User
	result := db.Where("user_id = ?", userID).First(&user)
	
	// 检查查询结果
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found: %s", userID)
		}
		return nil, fmt.Errorf("failed to get user: %w", result.Error)
	}
	
	return &user, nil
}
