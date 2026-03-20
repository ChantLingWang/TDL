package auth_token

import (
	"context"
	"log"
	"net/http"
	"strings"

	"chat_service/app/infrastructure/grpc"
	"chat_service/app/services"

	"github.com/gin-gonic/gin"
)

// Auth 认证中间件
// 从 Authorization Header 中提取并验证 Token，将用户信息存入 Context
// 适用于需要认证的 HTTP 和 WebSocket 路由
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从 Header 获取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		// 2. 解析 Token (Bearer xxx)
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}

		// 3. 通过 gRPC 调用 auth_service 验证 Token
		authClient := grpc.GetAuthClient()
		resp, err := authClient.VerifyToken(context.Background(), token)
		if err != nil || !resp.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": resp.Message})
			c.Abort()
			return
		}

		// 4. 从响应中获取完整的用户信息，存入 Context
		userInfo := &services.UserInfo{
			UserID:   resp.UserId,
			Username: resp.Username,
			Email:    resp.Email,
		}
		c.Set("userInfo", userInfo)

		log.Printf("User authenticated: %s (%s)", userInfo.Username, userInfo.UserID)

		// 5. 认证通过，继续处理请求
		c.Next()
	}
}
