package handlers

import (
	"fmt"
	"net/http"
	"time"

	"chat_service/app/database/pgsql"
	"chat_service/app/services"

	"github.com/gin-gonic/gin"
)

const aiAssistantID = "ai-assistant"

type CreateGroupRequest struct {
	GroupName string `json:"group_name" binding:"required"`
	CreatorID string `json:"creator_id" binding:"required"`
	GroupType string `json:"group_type"`
}

type JoinGroupRequest struct {
	GroupID string `json:"group_id" binding:"required"`
	UserID  string `json:"user_id" binding:"required"`
}

func CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.GroupType == "" {
		req.GroupType = "normal"
	}

	service := pgsql.NewUserGroupService(pgsql.GetDBManager())

	var groupID string
	if req.GroupType == "ai" {
		groupID = fmt.Sprintf("ai_%d", time.Now().UnixNano())
	} else {
		var err error
		groupID, err = service.GenerateGroupID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate group ID"})
			return
		}
	}

	group, err := service.CreateGroup(groupID, req.GroupName, req.CreatorID, req.GroupType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}

	if err := service.AddUserToGroup(req.CreatorID, groupID); err != nil {
		service.DeleteGroup(groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add creator to group"})
		return
	}

	// 自动创建会话记录，使未读统计生效
	convSvc := services.GetConversationService()
	_ = convSvc.MarkMessageAsRead(c.Request.Context(), req.CreatorID, groupID)

	if req.GroupType == "ai" {
		if err := service.AddUserToGroup(aiAssistantID, groupID); err != nil {
			_ = err
		}
	}

	c.JSON(http.StatusCreated, group)
}

func JoinGroup(c *gin.Context) {
	var req JoinGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	service := pgsql.NewUserGroupService(pgsql.GetDBManager())
	if err := service.AddUserToGroup(req.UserID, req.GroupID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动创建会话记录
	convSvc := services.GetConversationService()
	_ = convSvc.MarkMessageAsRead(c.Request.Context(), req.UserID, req.GroupID)

	c.JSON(http.StatusOK, gin.H{"message": "User added to group successfully"})
}

func GetUserGroups(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	service := pgsql.NewUserGroupService(pgsql.GetDBManager())
	groups, err := service.GetUserGroups(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user groups"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"groups": groups})
}
