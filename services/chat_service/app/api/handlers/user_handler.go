package handlers

import (

	"chat_service/app/infrastructure/grpc"

	"github.com/gin-gonic/gin"
)

// GetUser 获取用户信息（通过 gRPC 向 auth service 查询）
func GetUser(c *gin.Context) {
	userID := c.Param("user_id")

	authClient := grpc.GetAuthClient()
	resp, err := authClient.GetUserByID(c, userID)
	if err != nil || !resp.Found {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}

	c.JSON(200, gin.H{
		"user_id":  resp.UserId,
		"username": resp.Username,
		"email":    resp.Email,
		"status":   resp.Status,
	})
}
