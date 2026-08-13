package handlers

import (
	"net/http"
	"sort"

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
			"unread_counts":      map[string]int{},
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
			"unread_counts":      map[string]int{},
			"last_offline_time":  lastOfflineTime,
			"messages":           []interface{}{},
		})
		return
	}

	// 只统计用户当前仍是成员的群会话，防止被移出群后仍能通过残留会话记录读到未读消息
	groupSvc := pgsql.NewUserGroupService(pgsql.GetDBManager())
	myGroups, err := groupSvc.GetUserGroups(userInfo.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	memberOf := make(map[string]struct{}, len(myGroups))
	for _, g := range myGroups {
		memberOf[g.GroupID] = struct{}{}
	}
	authorized := make([]string, 0, len(conversationIDs))
	for _, convID := range conversationIDs {
		if _, ok := memberOf[convID]; ok {
			authorized = append(authorized, convID)
			continue
		}
		// 不在成员列表里的会话 ID：私聊会话直接保留；是群但已不是成员则丢弃
		if _, groupErr := groupSvc.GetGroupByID(convID); groupErr != nil {
			authorized = append(authorized, convID)
		}
	}
	conversationIDs = authorized

	mongoService := mongodb.GetGroupMessageHistoryService()
	unreadCounts := make(map[string]int, len(conversationIDs))
	totalCount := 0
	var allMessages []mongodb.Message
	for _, convID := range conversationIDs {
		// 以「最后已读时间」和「最后离线时间」的较晚者作为该会话的未读分界
		cutoff := lastOfflineTime
		lastRead, readErr := conversationService.GetLastReadTime(
			c.Request.Context(), userInfo.UserID, convID,
		)
		if readErr == nil && !lastRead.IsZero() && lastRead.Unix() > cutoff {
			cutoff = lastRead.Unix()
		}

		msgs, count, mongoErr := mongoService.GetUnreadMessages(
			[]string{convID}, cutoff, 1001,
		)
		if mongoErr != nil {
			continue
		}
		unreadCounts[convID] = count
		totalCount += count
		allMessages = append(allMessages, msgs...)
	}

	sort.Slice(allMessages, func(i, j int) bool {
		return allMessages[i].Timestamp.After(allMessages[j].Timestamp)
	})

	displayCount := totalCount
	if displayCount > MaxUnreadCount {
		displayCount = MaxUnreadCount
	}
	responseMessages := allMessages
	if len(allMessages) > MaxDisplayCount {
		responseMessages = allMessages[:MaxDisplayCount]
	}

	c.JSON(http.StatusOK, gin.H{
		"total_unread_count": displayCount,
		"unread_counts":      unreadCounts,
		"last_offline_time":  lastOfflineTime,
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

	// 越权校验：JWT 用户只能读取自己参与的会话。
	// 内部服务（X-Internal-Key）没有 userInfo，视为可信，跳过校验。
	if userInfoVal, exists := c.Get("userInfo"); exists {
		userID := userInfoVal.(*services.UserInfo).UserID
		if isGroup {
			ok, checkErr := groupSvc.IsUserInGroup(userID, convID)
			if checkErr != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		} else if !mongodb.IsPrivateParticipant(convID, userID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	mongoSvc := mongodb.GetGroupMessageHistoryService()
	msgs, err := mongoSvc.GetHistoryMessages(convID, req.Cursor, req.StartTime, req.EndTime, req.Keyword, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !isGroup {
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
