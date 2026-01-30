package pgsql

import (
	"context"
	"time"

	"user_service/app/database/pgsql/model"
	"user_service/app/database/pgsql/query"
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
func (service *UserService) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	// 使用 GEN 的 Query 对象
	u := query.User
	
	// 获取用户数据
	user, err := u.WithContext(ctx).Where(u.UserID.Eq(userID)).First()
	
	// 检查查询结果
	if err != nil {
		return nil, err
	}
	
	return user, nil
}


// CreateUser 创建新用户
func (service *UserService) CreateUser(ctx context.Context, userID, username, email string) error {
	// 创建用户对象
	user := model.User{
		UserID:       userID,
		Username:     username,
		Email:        email,
		RegisterTime: time.Now(),
	}
	
	// 使用 GEN 保存到数据库
	err := query.User.WithContext(ctx).Create(&user)
	if err != nil {
		return err
	}
	
	return nil
}
