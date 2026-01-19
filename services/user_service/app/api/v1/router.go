package v1

import (
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
	// 创建API v1路由组
	v1 := r.Engine.Group("/api/v1")

	// 用户相关路由
	users := v1.Group("/users")
	{
		// 根据用户ID获取用户信息
		users.GET("/:user_id", GetUser)
		// 初始化测试用户 (POST /api/v1/users/:user_id/init)
		users.POST("/:user_id/init", InitTestUser)
		// 获取用户群组
		users.GET("/:user_id/groups", GetUserGroups)
	}

	// 群组相关路由
	groups := v1.Group("/groups")
	{
		// 创建群组 (POST /api/v1/groups)
		groups.POST("", CreateGroup)
		// 加入群组 (POST /api/v1/groups/join)
		groups.POST("/join", JoinGroup)
	}

	// WebSocket 路由
	// 注意：WebSocket 是 GET 请求
	v1.GET("/ws", ConnectWebSocket)

	return v1
}

// GetRouter 获取路由器实例
func GetRouter() *Router {
	return NewRouter()
}
