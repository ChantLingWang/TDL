package database

import "gorm.io/gorm"

// User 用户模型
type User struct {
	UserID       string `gorm:"primaryKey"`
	Username string `gorm:"not null"`
	Email    string `gorm:"uniqueIndex;not null"`
	// 可以根据需要添加其他用户字段
}

// Group 组群模型
type Group struct {
	GroupID string `gorm:"primaryKey"`
	Name string `gorm:"not null"`
	MessageHistory string `gorm:"foreignKey:GroupID;references:GroupID"`
	Users []User `gorm:"many2many:user_groups;"`
	// 可以根据需要添加其他组群字段
}

// UserGroup 用户组群关联模型（多对多关系）
type UserGroup struct {
	UserID  string `gorm:"primaryKey"`
	GroupID string `gorm:"primaryKey"`
	// 可以根据需要添加其他关联字段，如加入时间等
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

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &Group{}, &UserGroup{})
}
