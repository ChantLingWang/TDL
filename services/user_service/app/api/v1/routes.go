package user

import (
	"chant/user_service/app/api/utils"
	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户相关的路由
func RegisterUserRoutes(engine *gin.Engine) {
	// WebSocket路由，支持房间ID参数
	engine.GET("/wss/rooms/:room_id", utils.WebSocketHandler)
	
	// 默认WebSocket路由（使用默认房间）
	engine.GET("/wss", utils.WebSocketHandler)
}
