package model

import (
	"time"
	
	"gorm.io/gorm"
)


// User 用户模型
type User struct {
	RegisterTime time.Time `gorm:"not null"`
	UserID       string `gorm:"primaryKey"`
	Username string `gorm:"not null"`
	Email    string `gorm:"uniqueIndex;not null"`
	Groups   []Group `gorm:"many2many:user_groups;"`
	Tempchat []TempChat `gorm:"foreignKey:UserID;references:UserID"`
	PrivateChat []PrivateChat `gorm:"foreignKey:UserID;references:UserID"`
}


// Group 组群模型
type Group struct {
	CreateTime    time.Time `gorm:"not null"`
	GroupID       string    `gorm:"primaryKey"`
	GroupName     string    `gorm:"not null"`
	Users         []User    `gorm:"many2many:user_groups;"`
	CreateByUserID string   `gorm:"not null"` // 创建者ID (单人)
	Managers      []User    `gorm:"many2many:group_managers;"` // 管理员列表 (多人)
}


// UserGroup 用户组群关联模型（多对多关系）
type UserGroup struct {
	UserID  string `gorm:"primaryKey"`
	GroupID string `gorm:"primaryKey"`
	// 可以根据需要添加其他关联字段，如加入时间等
}


// 定义私有chat模型
type PrivateChat struct {
	UserID string `gorm:"primaryKey"`
	AddTime time.Time `gorm:"primaryKey"`
}


// 定义临时chat模型
type TempChat struct {
	UserID string `gorm:"primaryKey"`
	Source string `gorm:"primaryKey"`
}


// Conversation 会话模型（用于跟踪用户对会话的已读状态）
type Conversation struct {
	UserID           string    `gorm:"primaryKey"`        // 用户ID
	ConversationID   string    `gorm:"primaryKey"`        // 会话ID (群ID 或 userA_userB)
	ConversationType string    `gorm:"not null"`          // 会话类型: "private" 或 "group"
	LastReadTime     time.Time // 最后已读时间
	UpdateTime       time.Time // 更新时间
}

// TableName 指定User表名
func (User) TableName() string {
	return "users"
}

// TableName 指定Group表名
func (Group) TableName() string {
	return "groups"
}

// TableName 指定UserGroup表名
func (UserGroup) TableName() string {
	return "user_groups"
}

// TableName 指定PrivateChat表名
func (PrivateChat) TableName() string {
	return "private_chats"
}

// TableName 指定TempChat表名
func (TempChat) TableName() string {
	return "temp_chats"
}

// TableName 指定Conversation表名
func (Conversation) TableName() string {
	return "conversations"
}

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &Group{}, &UserGroup{}, &PrivateChat{}, &TempChat{}, &Conversation{})
}
