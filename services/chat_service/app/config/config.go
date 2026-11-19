package config

import (
	"log"
	"os"
	"strings"

	"infrastructure_sdk/config"
)

// ServerCfg HTTP / WebSocket 服务配置
type ServerCfg struct {
	Port string `yaml:"port"`
}

// KafkaConfig 生产者 / 消费者配置
type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`
	GroupID string   `yaml:"group_id"`
}

// Config gateway 的全局配置；gateway 不持有任何数据库配置。
type Config struct {
	Server ServerCfg   `yaml:"server"`
	Kafka  KafkaConfig `yaml:"kafka"`

	// AI 用户 ID；gateway 把用户消息投递给它
	AIUserID string `yaml:"ai_user_id"`

	// CORS 允许的 Origin（为空则不输出 CORS 头）
	CORSAllowedOrigin string `yaml:"cors_allowed_origin"`
	// WebSocket 允许的 Origin 列表（逗号分隔，为空时仅允许无 Origin 请求）
	WSAllowedOrigins string `yaml:"ws_allowed_origins"`
}

var (
	ServerConfig        ServerCfg
	KafkaConfigInstance KafkaConfig
	AIUserID            string
	CORSAllowedOrigin   string
	WSAllowedOrigins    string
)

// InitConfig 初始化全局配置。
func InitConfig(path string) {
	var globalConfig Config
	if err := config.LoadConfig(path, &globalConfig); err != nil {
		log.Fatalf("Failed to load config from %s: %v", path, err)
	}

	ServerConfig = globalConfig.Server
	AIUserID = globalConfig.AIUserID
	if AIUserID == "" {
		AIUserID = "ai-assistant"
	}

	// Kafka brokers 支持逗号分隔的环境变量覆盖
	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		globalConfig.Kafka.Brokers = strings.Split(brokers, ",")
	}

	// 每台 gateway 使用独立 consumer group，保证每个实例都能收到全量 AI 回复并推给本地 WS。
	groupID := os.Getenv("CHAT_GROUP_ID")
	if groupID == "" {
		hostname, _ := os.Hostname()
		groupID = globalConfig.Kafka.GroupID + "_" + hostname
	}
	KafkaConfigInstance = KafkaConfig{
		Brokers: globalConfig.Kafka.Brokers,
		Topic:   globalConfig.Kafka.Topic,
		GroupID: groupID,
	}

	CORSAllowedOrigin = globalConfig.CORSAllowedOrigin
	WSAllowedOrigins = globalConfig.WSAllowedOrigins

	subEnv(&ServerConfig.Port, "SERVER_PORT")
	subEnv(&AIUserID, "AI_USER_ID")
	subEnv(&CORSAllowedOrigin, "CORS_ALLOWED_ORIGIN")
	subEnv(&WSAllowedOrigins, "WS_ALLOWED_ORIGINS")
}

// subEnv 如果环境变量存在则覆盖指针指向的值。
func subEnv(ptr *string, envKey string) {
	if v := os.Getenv(envKey); v != "" {
		*ptr = v
	}
}
