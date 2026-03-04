package config

import (
	"log"
	"time"

	"infrastructure_sdk/config"
)

// KafkaConfig Kafka连接配置
type KafkaConfig struct {
	Brokers  []string `yaml:"brokers"`
	GroupID  string   `yaml:"group_id"`
	Topic    string   `yaml:"topic"`
	ClientID string   `yaml:"client_id"`
}

// Config 定义全局配置结构
type Config struct {
	DefaultTimeout      time.Duration `yaml:"default_timeout"`
	DefaultRetryCount   int           `yaml:"default_retry_count"`
	CompensationTimeout time.Duration `yaml:"compensation_timeout"`
	SnowflakeNodeID     int64         `yaml:"snowflake_node_id"`

	Kafka KafkaConfig `yaml:"kafka"`

	Database struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DBName   string `yaml:"dbname"`
		SSLMode  string `yaml:"sslmode"`
		TimeZone string `yaml:"timezone"`
	} `yaml:"database"`

	Templates struct {
		Path string `yaml:"path"`
	} `yaml:"templates"`

	Saga struct {
		ExecutionTimeout   time.Duration `yaml:"execution_timeout"`
		MaxConcurrentSagas int           `yaml:"max_concurrent_sagas"`
		EnableStepRetry    bool          `yaml:"enable_step_retry"`
	} `yaml:"saga"`

	Monitoring struct {
		EnableHealthCheck bool `yaml:"enable_health_check"`
		EnableMetrics     bool `yaml:"enable_metrics"`
	} `yaml:"monitoring"`
}

// GlobalConfig 全局配置实例
var GlobalConfig Config

// InitGlobalConfig 初始化全局配置
func InitGlobalConfig(path string) {
	// 加载配置文件
	if err := config.LoadConfig(path, &GlobalConfig); err != nil {
		log.Fatalf("Failed to load config from %s: %v", path, err)
	}
}
