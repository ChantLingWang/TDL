package v1

import (
	"net/http"
	"time"

	"user_service/app/database/pgsql"
	"user_service/app/database/pgsql/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateGroupRequest 创建群组请求
type CreateGroupRequest struct {
	GroupName string `json:"group_name" binding:"required"`
	CreatorID string `json:"creator_id" binding:"required"`
}

// JoinGroupRequest 加入群组请求
type JoinGroupRequest struct {
	GroupID string `json:"group_id" binding:"required"`
	UserID  string `json:"user_id" binding:"required"`
}

// CreateGroup 创建群组
func CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	service := pgsql.NewUserGroupService(pgsql.GetDBManager())

	// 1. 创建群组
	groupID := uuid.New().String()
	group, err := service.CreateGroup(groupID, req.GroupName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}

	// 2. 将创建者加入群组
	if err := service.AddUserToGroup(req.CreatorID, groupID); err != nil {
		// 回滚：删除创建的群组
		service.DeleteGroup(groupID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add creator to group"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Group created successfully",
		"group_id":   group.GroupID,
		"group_name": group.GroupName,
		"created_at": group.CreateTime,
	})
}

// JoinGroup 加入群组
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

	c.JSON(http.StatusOK, gin.H{
		"message": "User added to group successfully",
	})
}

// GetUserGroups 获取用户所在的群组
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

	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
	})
}

// InitTestUser 初始化测试用户（方便测试用）
func InitTestUser(c *gin.Context) {
	userID := c.Param("user_id")
	username := c.Query("username")
	if username == "" {
		username = "Test User " + userID
	}

	db := pgsql.GetDBManager().GetDB()

	user := model.User{
		UserID:       userID,
		Username:     username,
		Email:        userID + "@example.com",
		RegisterTime: time.Now(),
	}

	// FirstOrCreate
	if err := db.Where(model.User{UserID: userID}).FirstOrCreate(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User initialized", "user": user})
}
