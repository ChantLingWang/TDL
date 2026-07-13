package routes

import (
	"chat_service/app/api/handlers"
	"chat_service/app/api/websocket"
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
	r.Engine.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 内部接口，无需鉴权（供 ai_service 拉取历史消息）
	r.Engine.GET("/api/v1/messages/history", handlers.GetMessageHistory)

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

func GetRouter() *Router {
	return NewRouter()
}
