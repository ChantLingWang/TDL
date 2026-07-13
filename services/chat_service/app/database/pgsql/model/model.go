package model

import (
	"time"

	"gorm.io/gorm"
)

// Group 组群模型
type Group struct {
	GroupID        string    `gorm:"primaryKey" json:"group_id"`
	GroupName      string    `gorm:"not null" json:"group_name"`
	GroupType      string    `gorm:"default:normal" json:"group_type"` // "ai" | "normal"
	CreateByUserID string    `gorm:"not null" json:"create_by_user_id"`
	CreateTime     time.Time `gorm:"not null" json:"create_time"`
}

// UserGroup 用户组群关联模型
type UserGroup struct {
	UserID  string `gorm:"primaryKey" json:"user_id"`
	GroupID string `gorm:"primaryKey" json:"group_id"`
}

// PrivateChat 私有chat模型
type PrivateChat struct {
	UserID  string    `gorm:"primaryKey" json:"user_id"`
	AddTime time.Time `gorm:"primaryKey" json:"add_time"`
}

// TempChat 临时chat模型
type TempChat struct {
	UserID string `gorm:"primaryKey" json:"user_id"`
	Source string `gorm:"primaryKey" json:"source"`
}

// Conversation 会话模型（已读状态追踪）
type Conversation struct {
	UserID           string    `gorm:"primaryKey" json:"user_id"`
	ConversationID   string    `gorm:"primaryKey" json:"conversation_id"`
	ConversationType string    `gorm:"not null" json:"conversation_type"`
	LastReadTime     time.Time `json:"last_read_time"`
	UpdateTime       time.Time `json:"update_time"`
}

func (Group) TableName() string        { return "groups" }
func (UserGroup) TableName() string    { return "user_groups" }
func (PrivateChat) TableName() string  { return "private_chats" }
func (TempChat) TableName() string     { return "temp_chats" }
func (Conversation) TableName() string { return "conversations" }

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&Group{}, &UserGroup{}, &PrivateChat{}, &TempChat{}, &Conversation{})
}
