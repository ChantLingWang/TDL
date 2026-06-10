package handlers

import (
	chatconst "chat_service/app/const"
	"chat_service/app/database/mongodb"
	"chat_service/app/middleware/get_user_chat_info"
	"chat_service/app/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 分页常量
const (
	DefaultMessageLimit   = 100  // 默认消息数量
	MaxUnreadCount       = 1000 // 最大未读消息数
	MaxDisplayCount      = 100  // 最大显示消息数
)

// MarkReadRequest 标记已读请求
type MarkReadRequest struct {
	ConversationID string `json:"conversation_id" binding:"required"`
}

// GetHistoryRequest 获取历史消息请求
type GetHistoryRequest struct {
	ConversationID string `form:"conversation_id" binding:"required"`
	Cursor          int64  `form:"cursor"`
	StartTime       int64  `form:"start_time"`
	EndTime         int64  `form:"end_time"`
	Keyword         string `form:"keyword"`
	Limit           int    `form:"limit,default=100"`
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

// GetMessages 获取消息列表
// GET /api/v1/messages?conversation_id=xxx&limit=30
func GetMessages(c *gin.Context) {
	// 获取当前用户
	userInfoVal, _ := c.Get("userInfo")
	userInfo := userInfoVal.(*services.UserInfo)

	// 1. 获取用户的 last_offline_time（全局最后阅读时间）
	lastOfflineTime, err := get_user_chat_info.GetLastOfflineTime(userInfo.UserID, userInfo.Username)
	if err != nil || lastOfflineTime == 0 {
		// 如果没有离线时间，默认返回空
		c.JSON(http.StatusOK, gin.H{
			"total_unread_count": 0,
			"last_offline_time":  0,
			"messages":           []interface{}{},
		})
		return
	}

	// 2. 获取用户所有会话 ID
	conversationService := services.GetConversationService()
	conversationIDs, err := conversationService.GetUserConversationIDs(
		c.Request.Context(),
		userInfo.UserID,
	)
	if err != nil || len(conversationIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"total_unread_count": 0,
			"last_offline_time":  lastOfflineTime,
			"messages":           []interface{}{},
		})
		return
	}

	// 3. 查询未读消息（限制 1001 条，用于判断是否超过 1000）
	mongoService := mongodb.GetGroupMessageHistoryService()
	messages, totalCount, err := mongoService.GetUnreadMessages(conversationIDs, lastOfflineTime, 1001)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 4. 处理返回值
	// 如果总数超过 1000，限制为 1000
	displayCount := totalCount
	if displayCount > MaxUnreadCount {
		displayCount = MaxUnreadCount
	}

	// 只返回前 100 条
	responseMessages := messages
	if len(messages) > MaxDisplayCount {
		responseMessages = messages[:MaxDisplayCount]
	}

	// 直接返回消息列表
	c.JSON(http.StatusOK, gin.H{
		"total_unread_count": displayCount,
		"messages":           responseMessages,
	})
}

// GetMessageHistory 获取历史消息
// GET /api/v1/messages/history?conversation_id=xxx&start_time=xxx&end_time=xxx&keyword=xxx
func GetMessageHistory(c *gin.Context) {
	// 获取当前用户
	userInfoVal, _ := c.Get("userInfo")
	userInfo := userInfoVal.(*services.UserInfo)

	// 验证和解析参数
	var req GetHistoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// cursor 和 start_time/end_time 不能同时传
	if req.Cursor > 0 && (req.StartTime > 0 || req.EndTime > 0) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cursor and time range cannot be used together"})
		return
	}

	// 必须传 cursor 或 start_time/end_time 之一
	if req.Cursor == 0 && req.StartTime == 0 && req.EndTime == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cursor or time range is required"})
		return
	}

	// 设置默认值
	if req.Limit <= 0 {
		req.Limit = DefaultMessageLimit
	}

	// 查询会话类型（群聊还是私聊）
	conversationService := services.GetConversationService()
	conversation, err := conversationService.GetConversation(c.Request.Context(), userInfo.UserID, req.ConversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "conversation not found"})
		return
	}

	// 根据会话类型调用不同的服务
	var messages []interface{}

	if conversation.ConversationType == chatconst.ConversationTypeGroup {
		// 群聊消息
		mongoService := mongodb.GetGroupMessageHistoryService()
		msgs, err := mongoService.GetHistoryMessages(req.ConversationID, req.Cursor, req.StartTime, req.EndTime, req.Keyword, req.Limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// 转换为 interface{} 切片
		for _, msg := range msgs {
			messages = append(messages, msg)
		}
	} else {
		// 私聊消息
		mongoService := mongodb.GetPrivateMessageHistoryService()
		msgs, err := mongoService.GetHistoryMessages(req.ConversationID, req.Cursor, req.StartTime, req.EndTime, req.Keyword, req.Limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, msg := range msgs {
			messages = append(messages, msg)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
	})
}
