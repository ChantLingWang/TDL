package handlers

import (
	"chat_service/app/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// MarkReadRequest 标记已读请求
type MarkReadRequest struct {
	ConversationID string `json:"conversation_id" binding:"required"`
}

// MarkMessagesAsRead 标记消息已读（更新最后阅读时间）
// POST /api/v1/messages/read
func MarkMessagesAsRead(c *gin.Context) {
	// 获取当前用户
	userInfoVal, _ := c.Get("userInfo")
	userInfo := userInfoVal.(*services.UserInfo)

	var req MarkReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 调用服务标记已读
	conversationService := services.GetConversationService()
	err := conversationService.MarkMessageAsRead(
		c.Request.Context(),
		userInfo.UserID,
		req.ConversationID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
	})
}

// GetMessages 获取消息列表（同时返回未读状态和消息内容）
// GET /api/v1/messages?conversation_id=xxx&limit=50
func GetMessages(c *gin.Context) {
	// 获取当前用户
	userInfoVal, _ := c.Get("userInfo")
	userInfo := userInfoVal.(*services.UserInfo)

	conversationID := c.Query("conversation_id")

	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id is required"})
		return
	}

	// 1. 获取用户最后阅读时间
	conversationService := services.GetConversationService()
	lastReadTime, err := conversationService.GetLastReadTime(
		c.Request.Context(),
		userInfo.UserID,
		conversationID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 2. TODO: 从 MongoDB 查询 lastReadTime 之后的所有未读消息
	// 根据 conversation_id 判断是私聊还是群聊，然后查询对应 MongoDB 集合

	// 临时返回空消息列表，后续完善 MongoDB 查询逻辑
	c.JSON(http.StatusOK, gin.H{
		"conversation_id": conversationID,
		"last_read_time":  lastReadTime,
		"messages":        []interface{}{},
	})
}
