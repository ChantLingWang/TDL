package pgsql

import (
	"context"
	"time"
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


// GetUserByID 根据用户ID获取用户信息
func (service *UserService) GetUserByID(ctx context.Context, userID string) (*User, error) {
	db := service.dbManager.GetDB()
	
	// 获取用户数据
	var user User
	result := db.Where("user_id = ?", userID).First(&user)
	
	// 检查查询结果
	if result.Error != nil {
		return nil, result.Error // 直接返回原始错误
	}
	
	return &user, nil
}


// CreateUser 创建新用户
func (service *UserService) CreateUser(ctx context.Context, userID, username, email string) error {
	db := service.dbManager.GetDB()
	
	// 创建用户对象
	user := User{
		UserID:       userID,
		Username:     username,
		Email:        email,
		RegisterTime: time.Now(),
	}
	
	// 保存到数据库
	result := db.Create(&user)
	if result.Error != nil {
		return result.Error
	}
	
	return nil
}

