package main

import (
	"os"
	"strings"
	"time"
)

// GlobalConfig 全局配置
var GlobalConfig = struct {
	// 默认超时时间
	DefaultTimeout time.Duration

	// 默认重试次数
	DefaultRetryCount int

	// 补偿操作超时时间
	CompensationTimeout time.Duration

	// Kafka相关配置
	Kafka struct {
		Brokers  []string
		GroupID  string
		Topic    string
		ClientID string
	}

	// 模板相关配置
	Templates struct {
		// 模板文件路径
		Path string
	}

	// Saga执行配置
	Saga struct {
		// Saga执行超时时间
		ExecutionTimeout time.Duration

		// 最大并发Saga数量
		MaxConcurrentSagas int

		// 是否启用步骤重试
		EnableStepRetry bool
	}

	// 监控配置
	Monitoring struct {
		// 是否启用健康检查
		EnableHealthCheck bool

		// 是否启用性能指标
		EnableMetrics bool
	}
}{
	DefaultTimeout:      30 * time.Second,
	DefaultRetryCount:   3,
	CompensationTimeout: 15 * time.Second,

	Kafka: struct {
		Brokers  []string
		GroupID  string
		Topic    string
		ClientID string
	}{
		Brokers:  []string{"localhost:9092"},
		GroupID:  "orchestrator-group",
		Topic:    "saga-events",
		ClientID: "orchestrator-service",
	},

	Templates: struct {
		Path string
	}{
		Path: "templates/example_yaml",
	},

	Saga: struct {
		ExecutionTimeout   time.Duration
		MaxConcurrentSagas int
		EnableStepRetry    bool
	}{
		ExecutionTimeout:   5 * time.Minute,
		MaxConcurrentSagas: 100,
		EnableStepRetry:    true,
	},

	Monitoring: struct {
		EnableHealthCheck bool
		EnableMetrics     bool
	}{
		EnableHealthCheck: true,
		EnableMetrics:     true,
	},
}

// InitGlobalConfig 初始化全局配置
func InitGlobalConfig() {
	// 从环境变量覆盖Kafka配置
	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		GlobalConfig.Kafka.Brokers = strings.Split(brokers, ",")
	}

	if groupID := os.Getenv("KAFKA_GROUP_ID"); groupID != "" {
		GlobalConfig.Kafka.GroupID = groupID
	}

	if topic := os.Getenv("KAFKA_TOPIC"); topic != "" {
		GlobalConfig.Kafka.Topic = topic
	}

	if clientID := os.Getenv("KAFKA_CLIENT_ID"); clientID != "" {
		GlobalConfig.Kafka.ClientID = clientID
	}

}
