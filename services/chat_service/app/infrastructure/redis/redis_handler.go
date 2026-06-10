package redis

import (
	"context"
)

// RedisHandler Redis 业务处理
type RedisHandler struct {
	client *RedisClient
}

// NewRedisHandler 创建 Redis 业务处理器
func NewRedisHandler() *RedisHandler {
	return &RedisHandler{
		client: GetRedisClient(),
	}
}

// HSet 设置 Hash 字段
func (h *RedisHandler) HSet(ctx context.Context, key, field, value string) error {
	return h.client.GetClient().HSet(ctx, key, field, value).Err()
}

// HGet 获取 Hash 字段
func (h *RedisHandler) HGet(ctx context.Context, key, field string) (string, error) {
	return h.client.GetClient().HGet(ctx, key, field).Result()
}

// Del 删除 Key
func (h *RedisHandler) Del(ctx context.Context, keys ...string) error {
	return h.client.GetClient().Del(ctx, keys...).Err()
}

// Exists 检查 Key 是否存在
func (h *RedisHandler) Exists(ctx context.Context, key string) (bool, error) {
	result, err := h.client.GetClient().Exists(ctx, key).Result()
	return result > 0, err
}
