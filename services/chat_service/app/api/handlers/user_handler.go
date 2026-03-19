package handlers

import (
	"github.com/gin-gonic/gin"
	"chat_service/app/database/pgsql"
)

// GetUser 获取用户信息
func GetUser(c *gin.Context) {
	// 从URL参数中获取用户ID
	userID := c.Param("user_id")
	
	// 调用服务层获取用户信息
	user, err := pgsql.NewUserService(pgsql.GetDBManager()).GetUserByID(c, userID)
	if err != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	
	// 返回用户信息
	c.JSON(200, user)
}


// UpdateUser 更新用户信息
func UpdateUser(c *gin.Context) {
	
}


// DeleteUser 删除用户
func DeleteUser(c *gin.Context) {
	
}


// GetUsers 获取用户列表
func GetUsers(c *gin.Context) {
	
}
