package handlers

import (
	"net/http"

	"chat_service/app/infrastructure/grpc"

	"github.com/gin-gonic/gin"
)

// GetUser 代理 auth_service 的用户查询，gateway 不碰数据库。
func GetUser(c *gin.Context) {
	userID := c.Param("user_id")
	client := grpc.GetAuthClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth service unavailable"})
		return
	}

	resp, err := client.GetUserByID(c.Request.Context(), userID)
	if err != nil || resp == nil || !resp.Found {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":  resp.UserId,
		"username": resp.Username,
		"email":    resp.Email,
		"status":   resp.Status,
	})
}
