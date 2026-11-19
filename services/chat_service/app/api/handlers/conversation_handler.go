package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateConversation 只生成一个 conversation_id，不落库。
// 第一条消息到达时由 ai_service / storage 创建会话记录。
func CreateConversation(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"conversation_id": uuid.NewString()})
}
