package handlers

import (
	"fmt"
	"net/http"
	"time"

	"chat_service/app/database/pgsql"
	"chat_service/app/infrastructure/grpc"
	"chat_service/app/services"

	"github.com/gin-gonic/gin"
)

const (
	aiAssistantID = "ai-assistant"
	aiResearchID  = "ai-research"
)

type CreateGroupRequest struct {
	GroupName string `json:"group_name" binding:"required"`
	GroupType string `json:"group_type"`
}

type AddMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

func CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 身份一律取自登录态，忽略请求体里的伪造字段
	userInfo := c.MustGet("userInfo").(*services.UserInfo)
	if req.GroupType == "" {
		req.GroupType = "normal"
	}

	service := pgsql.NewUserGroupService(pgsql.GetDBManager())

	var groupID string
	isAI := req.GroupType == "ai" || req.GroupType == "ai-research"
	if isAI {
		groupID = fmt.Sprintf("ai_%d", time.Now().UnixNano())
	} else {
		var err error
		groupID, err = service.GenerateGroupID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate group ID"})
			return
		}
	}

	group, err := service.CreateGroup(groupID, req.GroupName, userInfo.UserID, req.GroupType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}

	if err := service.AddUserToGroup(userInfo.UserID, groupID); err != nil {
		service.DeleteGroup(groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add creator to group"})
		return
	}

	// 自动创建会话记录，使未读统计生效
	convSvc := services.GetConversationService()
	_ = convSvc.MarkMessageAsRead(c.Request.Context(), userInfo.UserID, groupID)

	if req.GroupType == "ai" {
		if err := service.AddUserToGroup(aiAssistantID, groupID); err != nil {
			_ = err
		}
	}
	if req.GroupType == "ai-research" {
		if err := service.AddUserToGroup(aiResearchID, groupID); err != nil {
			_ = err
		}
	}

	c.JSON(http.StatusCreated, group)
}

// AddGroupMember 群主将指定用户拉入群组。
// 成员管理只允许群主（groups.create_by_user_id）操作，杜绝凭群号自行入群。
func AddGroupMember(c *gin.Context) {
	userInfo := c.MustGet("userInfo").(*services.UserInfo)
	groupID := c.Param("group_id")

	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	service := pgsql.NewUserGroupService(pgsql.GetDBManager())
	group, err := service.GetGroupByID(groupID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	if group.CreateByUserID != userInfo.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the group creator can manage members"})
		return
	}

	// 校验目标用户真实存在，避免拉入无效 ID
	authResp, authErr := grpc.GetAuthClient().GetUserByID(c.Request.Context(), req.UserID)
	if authErr != nil || !authResp.Found {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := service.AddUserToGroup(req.UserID, groupID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动创建会话记录
	convSvc := services.GetConversationService()
	_ = convSvc.MarkMessageAsRead(c.Request.Context(), req.UserID, groupID)

	c.JSON(http.StatusOK, gin.H{"message": "member added"})
}

// GetGroupMembers 返回群成员 ID 列表，仅群成员可见。
func GetGroupMembers(c *gin.Context) {
	userInfo := c.MustGet("userInfo").(*services.UserInfo)
	groupID := c.Param("group_id")

	service := pgsql.NewUserGroupService(pgsql.GetDBManager())
	isMember, err := service.IsUserInGroup(userInfo.UserID, groupID)
	if err != nil || !isMember {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	members, err := service.GetGroupMembers(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"members": members})
}

// RemoveGroupMember 群主将成员移出群组；群主本人不可被移除。
func RemoveGroupMember(c *gin.Context) {
	userInfo := c.MustGet("userInfo").(*services.UserInfo)
	groupID := c.Param("group_id")
	targetUserID := c.Param("user_id")

	service := pgsql.NewUserGroupService(pgsql.GetDBManager())
	group, err := service.GetGroupByID(groupID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	if group.CreateByUserID != userInfo.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the group creator can manage members"})
		return
	}
	if targetUserID == group.CreateByUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot remove the group creator"})
		return
	}

	if err := service.RemoveUserFromGroup(targetUserID, groupID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "member removed"})
}

func GetUserGroups(c *gin.Context) {
	// 忽略路径参数，只返回当前登录用户的群组
	userInfo := c.MustGet("userInfo").(*services.UserInfo)

	service := pgsql.NewUserGroupService(pgsql.GetDBManager())
	groups, err := service.GetUserGroups(userInfo.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user groups"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"groups": groups})
}
