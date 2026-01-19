package services

import (
	"context"
	"encoding/json"
	"log"
)

// EventHandler 事件处理函数接口，统一管理所有事件处理函数
type EventHandler interface {
	// HandleUserRegisteredEvent 处理用户注册事件
	HandleUserRegisteredEvent(ctx context.Context, data json.RawMessage) error
	
	// TODO: 添加其他事件处理函数的接口定义
}

// HandleUserRegisteredEvent 处理用户注册事件的具体函数
func HandleUserRegisteredEvent(ctx context.Context, data json.RawMessage) error {
	// 直接使用项目中的现有用户模型
	// TODO: 导入并使用项目中的用户模型，如：User或UserEntity
	var userData map[string]interface{}
	if err := json.Unmarshal(data, &userData); err != nil {
		log.Printf("解析用户注册事件失败: %v", err)
		return err
	}
	
	// TODO: 在这里添加具体的业务逻辑
	// 例如：保存到数据库、发送邮件、触发其他事件等
	log.Printf("处理用户注册事件: %+v", userData)
	
	return nil
}