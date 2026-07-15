package handlers

import (
	"net/http"
	"strings"

	"chat_service/app/database/mongodb"
	"chat_service/app/database/pgsql"
	"chat_service/app/middleware/get_user_chat_info"
	"chat_service/app/services"

	"github.com/gin-gonic/gin"
)

const (
	DefaultMessageLimit = 100
	MaxUnreadCount      = 1000
	MaxDisplayCount     = 100
)

type MarkReadRequest struct {
	ConversationID string `json:"conversation_id" binding:"required"`
}

type GetHistoryRequest struct {
	ConversationID string `form:"conversation_id" binding:"required"`
	Cursor         int64  `form:"cursor"`
	StartTime      int64  `form:"start_time"`
	EndTime        int64  `form:"end_time"`
	Keyword        string `form:"keyword"`
	Limit          int    `form:"limit,default=100"`
}

func MarkMessagesAsRead(c *gin.Context) {
	userInfoVal, _ := c.Get("userInfo")
	userInfo := userInfoVal.(*services.UserInfo)

	var req MarkReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conversationService := services.GetConversationService()
	err := conversationService.MarkMessageAsRead(c.Request.Context(), userInfo.UserID, req.ConversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func GetMessages(c *gin.Context) {
	userInfoVal, _ := c.Get("userInfo")
	userInfo := userInfoVal.(*services.UserInfo)

	lastOfflineTime, err := get_user_chat_info.GetLastOfflineTime(userInfo.UserID, userInfo.Username)
	if err != nil || lastOfflineTime == 0 {
		c.JSON(http.StatusOK, gin.H{
			"total_unread_count": 0,
			"last_offline_time":  0,
			"messages":           []interface{}{},
		})
		return
	}

	conversationService := services.GetConversationService()
	conversationIDs, err := conversationService.GetUserConversationIDs(c.Request.Context(), userInfo.UserID)
	if err != nil || len(conversationIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"total_unread_count": 0,
			"last_offline_time":  lastOfflineTime,
			"messages":           []interface{}{},
		})
		return
	}

	mongoService := mongodb.GetGroupMessageHistoryService()
	messages, totalCount, err := mongoService.GetUnreadMessages(conversationIDs, lastOfflineTime, 1001)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	displayCount := totalCount
	if displayCount > MaxUnreadCount {
		displayCount = MaxUnreadCount
	}
	responseMessages := messages
	if len(messages) > MaxDisplayCount {
		responseMessages = messages[:MaxDisplayCount]
	}

	c.JSON(http.StatusOK, gin.H{
		"total_unread_count": displayCount,
		"messages":           responseMessages,
	})
}

func GetMessageHistory(c *gin.Context) {
	var req GetHistoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 {
		req.Limit = DefaultMessageLimit
	}

	// 根据 group_id 判断会话类型：查 groups 表的 group_type
	convID := req.ConversationID
	groupSvc := pgsql.NewUserGroupService(pgsql.GetDBManager())
	group, err := groupSvc.GetGroupByID(convID)

	isGroup := err == nil && group != nil
	isAI := isGroup && strings.HasPrefix(convID, "ai_")

	mongoSvc := mongodb.GetGroupMessageHistoryService()
	msgs, err := mongoSvc.GetHistoryMessages(convID, req.Cursor, req.StartTime, req.EndTime, req.Keyword, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// AI 会话需要按 conversation_id 过滤
	if isAI || isGroup {
	} else {
		// 私聊
		privSvc := mongodb.GetPrivateMessageHistoryService()
		msgs, _ = privSvc.GetHistoryMessages(convID, req.Cursor, req.StartTime, req.EndTime, req.Keyword, req.Limit)
	}

	messages := make([]interface{}, len(msgs))
	for i, msg := range msgs {
		messages[i] = msg
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}
