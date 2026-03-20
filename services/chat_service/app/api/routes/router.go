package routes

import (
	"chat_service/app/api/handlers"
	"chat_service/app/api/websocket"
	"chat_service/app/middleware/auth_token"

	"github.com/gin-gonic/gin"
)

// Router 用户路由器，类似于Python中的APIRouter
type Router struct {
	Engine *gin.Engine
}

// NewRouter 创建新的用户路由器
func NewRouter() *Router {
	return &Router{}
}

// SetupRoutes 设置路由
func (r *Router) SetupRoutes() *gin.RouterGroup {
	// 创建API v1路由组，并应用全局认证中间件
	v1 := r.Engine.Group("/api/v1")
	v1.Use(auth_token.Auth())

	// 用户相关路由
	users := v1.Group("/users")
	{
		// 根据用户ID获取用户信息
		users.GET("/:user_id", handlers.GetUser)
		// 初始化测试用户 (POST /api/v1/users/:user_id/init)
		users.POST("/:user_id/init", handlers.InitTestUser)
		// 获取用户群组
		users.GET("/:user_id/groups", handlers.GetUserGroups)
	}

	// 群组相关路由
	groups := v1.Group("/groups")
	{
		// 创建群组 (POST /api/v1/groups)
		groups.POST("", handlers.CreateGroup)
		// 加入群组 (POST /api/v1/groups/join)
		groups.POST("/join", handlers.JoinGroup)
	}

	// WebSocket 路由
	v1.GET("/ws", websocket.HandleWebSocket)

	// 消息相关路由
	messages := v1.Group("/messages")
	{
		// 获取消息列表（包含未读消息）
		messages.GET("", handlers.GetMessages)
		// 标记消息已读
		messages.POST("/read", handlers.MarkMessagesAsRead)
	}

	return v1
}

// GetRouter 获取路由器实例
func GetRouter() *Router {
	return NewRouter()
}
