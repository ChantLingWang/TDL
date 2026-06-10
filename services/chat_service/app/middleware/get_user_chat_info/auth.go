package get_user_chat_info

import (
	"context"
	"strconv"

	"chat_service/app/infrastructure/grpc"
	"chat_service/app/infrastructure/redis"

	"github.com/gin-gonic/gin"
)

// Middleware 获取用户聊天相关信息中间件
// 从 auth_token 中间件获取 userID，然后查询 Redis 缓存，
// 如果没有缓存则调用 gRPC 获取并存储到 Redis
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从 Context 获取 userInfo（由 auth_token 中间件设置）
		userInfoVal, _ := c.Get("userInfo")
		userInfo := userInfoVal.(*struct {
			UserID   string
			Username string
			Email    string
		})

		// 2. 构建 Redis Key: userid:username
		redisKey := userInfo.UserID + ":" + userInfo.Username

		// 3. 检查缓存是否存在
		redisHandler := redis.NewRedisHandler()
		cacheExists, _ := redisHandler.Exists(context.Background(), redisKey)

		// 4. 缓存不存在，调用 gRPC 获取
		if !cacheExists {
			// 调用 gRPC 获取 last_offline_time
			client := grpc.GetLastOfflineTimeClient()
			resp, err := client.GetLastOfflineTime(context.Background(), userInfo.UserID)
			if err == nil && resp.LastOfflineTime > 0 {
				// 存到 Redis Hash
				_ = redisHandler.HSet(
					context.Background(),
					redisKey,
					"last_offline_time",
					strconv.FormatInt(resp.LastOfflineTime, 10),
				)
			}
		}

		// 5. 继续处理请求
		c.Next()
	}
}

// GetLastOfflineTime 从 Redis 获取用户的 last_offline_time
func GetLastOfflineTime(userID, username string) (int64, error) {
	redisKey := userID + ":" + username
	redisHandler := redis.NewRedisHandler()

	value, err := redisHandler.HGet(context.Background(), redisKey, "last_offline_time")
	if err != nil || value == "" {
		// Redis 中没有，调用 gRPC 获取
		client := grpc.GetLastOfflineTimeClient()
		resp, err := client.GetLastOfflineTime(context.Background(), userID)
		if err != nil {
			return 0, err
		}
		return resp.LastOfflineTime, nil
	}

	return strconv.ParseInt(value, 10, 64)
}
