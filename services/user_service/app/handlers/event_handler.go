package handlers

import (
	"context"
	"log"

	"user_service/app/models"
	"user_service/app/database/pgsql"
	"gorm.io/gorm"
)

// UserEventHandler 用户事件处理器
type UserEventHandler struct {
	db         *gorm.DB
	userService *pgsql.UserService
}

// NewUserEventHandler 创建新的用户事件处理器
func NewUserEventHandler(db *gorm.DB) *UserEventHandler {
	dbManager := pgsql.GetDBManager()
	userService := pgsql.NewUserService(dbManager)
	
	return &UserEventHandler{
		db:         db,
		userService: userService,
	}
}

// HandleUserRegistered 处理用户注册事件
func (h *UserEventHandler) HandleUserRegistered(event models.UserRegisteredEvent) error {	
	// 创建上下文
	ctx := context.Background()
	
	// 将用户信息存储到数据库
	err := h.userService.CreateUser(ctx, event.UserID, event.Username, event.Email)
	if err != nil {
		log.Printf("创建用户失败: %v", err)
		return err
	}

	return nil
}
