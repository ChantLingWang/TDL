package routes

import (
	"net/http"

	"chat_service/app/api/handlers"
	"chat_service/app/api/websocket"
	"chat_service/app/config"
	"chat_service/app/middleware/auth_token"

	"github.com/gin-gonic/gin"
)

type Router struct {
	Engine *gin.Engine
}

func NewRouter() *Router {
	return &Router{}
}

func (r *Router) SetupRoutes() *gin.RouterGroup {
	// CORS：生产环境通过 CORS_ALLOWED_ORIGIN 配置白名单；为空则不输出 CORS 头
	r.Engine.Use(func(c *gin.Context) {
		if origin := config.CORSAllowedOrigin; origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Internal-Key")
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 健康检查
	r.Engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "chat_service"})
	})

	// 历史接口：支持前端 JWT 或 ai_service 内部密钥
	r.Engine.GET("/api/v1/messages/history", authOrInternal(), handlers.GetMessageHistory)

	v1 := r.Engine.Group("/api/v1")
	v1.Use(auth_token.Auth())

	users := v1.Group("/users")
	{
		users.GET("/:user_id", handlers.GetUser)
		users.GET("/:user_id/groups", handlers.GetUserGroups)
	}

	groups := v1.Group("/groups")
	{
		groups.POST("", handlers.CreateGroup)
		groups.POST("/join", handlers.JoinGroup)
	}

	v1.GET("/ws", websocket.HandleWebSocket)

	messages := v1.Group("/messages")
	{
		messages.GET("", handlers.GetMessages)
		messages.POST("/read", handlers.MarkMessagesAsRead)
	}

	return v1
}

// authOrInternal 允许两种调用方：
//  1. 前端：携带 JWT（走 auth_token.Auth）
//  2. ai_service：携带 X-Internal-Key（内部服务密钥）
func authOrInternal() gin.HandlerFunc {
	return func(c *gin.Context) {
		if key := c.GetHeader("X-Internal-Key"); key != "" && key == config.InternalAPIKey {
			c.Next()
			return
		}
		auth_token.Auth()(c)
	}
}

func GetRouter() *Router {
	return NewRouter()
}
