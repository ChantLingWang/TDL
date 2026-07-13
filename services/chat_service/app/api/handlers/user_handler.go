package handlers

import (
	"chat_service/app/services"

	"github.com/gin-gonic/gin"
)

// GetUser 返回当前请求的用户信息。
// 用户身份已由 auth_token 中间件验证并注入 context，不需要再次查 auth_service。
func GetUser(c *gin.Context) {
	userInfo, exists := c.Get("userInfo")
	if !exists {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	u := userInfo.(*services.UserInfo)
	c.JSON(200, gin.H{
		"user_id":  u.UserID,
		"username": u.Username,
		"email":    u.Email,
	})
}

// GetUserGroups 由 group_handler.go 处理，不受影响。
