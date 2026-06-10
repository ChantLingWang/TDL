package redis

import (
	"fmt"

	"chat_service/app/config"

	"github.com/redis/go-redis/v9"
)

// RedisClient Redis 客户端
type RedisClient struct {
	client *redis.Client
}

// 全局客户端实例
var (
	redisClientInstance *RedisClient
)

// GetRedisClient 获取 Redis 客户端单例
func GetRedisClient() *RedisClient {
	if redisClientInstance == nil {
		redisClientInstance = NewRedisClient()
	}
	return redisClientInstance
}

// NewRedisClient 创建新的 Redis 客户端
func NewRedisClient() *RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.RedisConfigInstance.Host, config.RedisConfigInstance.Port),
		Password: "",
		DB:       config.RedisConfigInstance.DB,
	})

	return &RedisClient{
		client: client,
	}
}

// GetClient 获取底层 redis.Client
func (r *RedisClient) GetClient() *redis.Client {
	return r.client
}

// Close 关闭连接
func (r *RedisClient) Close() error {
	return r.client.Close()
}
