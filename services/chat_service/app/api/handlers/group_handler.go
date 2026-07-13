package handlers

import (
	"fmt"
	"net/http"
	"time"

	"chat_service/app/database/pgsql"

	"github.com/gin-gonic/gin"
)

const aiAssistantID = "ai-assistant"

type CreateGroupRequest struct {
	GroupName string `json:"group_name" binding:"required"`
	CreatorID string `json:"creator_id" binding:"required"`
	GroupType string `json:"group_type"` // "ai" | "normal"，默认 "normal"
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

	// 生成 group_id：AI 群用 ai_ 前缀，普通群用 Sequence
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

	// 将创建者加入群组
	if err := service.AddUserToGroup(req.CreatorID, groupID); err != nil {
		service.DeleteGroup(groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add creator to group"})
		return
	}

	// AI 群自动加入 ai-assistant
	if req.GroupType == "ai" {
		if err := service.AddUserToGroup(aiAssistantID, groupID); err != nil {
			// ai 加入失败不回滚，只打日志
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

	// json tag 已在 model 上，直接返回 struct 即可
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}
