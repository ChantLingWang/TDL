package handlers

import (
	"net/http"

	"chat_service/app/services"

	"github.com/gin-gonic/gin"
)

// ListConversations 返回当前用户的所有 AI 会话
// GET /api/v1/conversations?type=ai
func ListConversations(c *gin.Context) {
	userInfo := c.MustGet("userInfo").(*services.UserInfo)
	convType := c.DefaultQuery("type", "ai")

	svc := services.GetConversationService()
	list, err := svc.ListConversations(c.Request.Context(), userInfo.UserID, convType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"conversations": list})
}

// CreateConversation 创建新的 AI 会话
// POST /api/v1/conversations  body: {"title": "新聊天"}
func CreateConversation(c *gin.Context) {
	userInfo := c.MustGet("userInfo").(*services.UserInfo)

	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Title = "新聊天"
	}

	svc := services.GetConversationService()
	conv, err := svc.CreateAIConversation(c.Request.Context(), userInfo.UserID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, conv)
}
