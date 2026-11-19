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
	if config.CORSAllowedOrigin != "" {
		r.Engine.Use(func(c *gin.Context) {
			c.Header("Access-Control-Allow-Origin", config.CORSAllowedOrigin)
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
		})
	}

	r.Engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "chat_gateway"})
	})

	v1 := r.Engine.Group("/api/v1")
	v1.Use(auth_token.Auth())

	// 用户资料代理到 auth_service，gateway 不直接查库。
	v1.GET("/users/:user_id", handlers.GetUser)

	// 只生成 conversation_id；会话内容在第一条消息时才由下游创建。
	v1.POST("/conversations", handlers.CreateConversation)

	v1.GET("/ws", websocket.HandleWebSocket)

	return v1
}

func GetRouter() *Router {
	return NewRouter()
}
