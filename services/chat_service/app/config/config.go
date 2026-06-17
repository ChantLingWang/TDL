package config

import (
	"log"
	"os"
	"strings"

	"infrastructure_sdk/config"
)

// 数据库配置
type DBConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"db_name"`
	SSLMode  string `yaml:"ssl_mode"`
	TimeZone string `yaml:"timezone"`

	// MongoDB连接池配置参数
	MaxPoolSize              int  `yaml:"max_pool_size"`
	MinPoolSize              int  `yaml:"min_pool_size"`
	MaxIdleTimeMS            int  `yaml:"max_idle_time_ms"`
	ServerSelectionTimeoutMS int  `yaml:"server_selection_timeout_ms"`
	ConnectTimeoutMS         int  `yaml:"connect_timeout_ms"`
	SocketTimeoutMS          int  `yaml:"socket_timeout_ms"`
	RetryWrites              bool `yaml:"retry_writes"`
	RetryReads               bool `yaml:"retry_reads"`
}

// 服务器配置
type ServerCfg struct {
	Port string `yaml:"port"`
}

// Kafka配置
type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`
	GroupID string   `yaml:"group_id"`
}

// Redis配置
type RedisConfig struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
	DB   int    `yaml:"db"`
}

// Config 定义全局配置结构
type Config struct {
	Server   ServerCfg   `yaml:"server"`
	Postgres DBConfig    `yaml:"postgres"`
	MongoDB  DBConfig    `yaml:"mongodb"`
	Kafka    KafkaConfig `yaml:"kafka"`
	Redis    RedisConfig `yaml:"redis"`
}

// 全局变量，保持原有变量名以减少代码修改
var (
	DataBaseConfig      DBConfig
	MongoDBConfig       DBConfig
	ServerConfig        ServerCfg
	KafkaConfigInstance KafkaConfig
	RedisConfigInstance RedisConfig
)

// InitConfig 初始化全局配置
func InitConfig(path string) {
	var globalConfig Config

	// 加载配置文件
	if err := config.LoadConfig(path, &globalConfig); err != nil {
		log.Fatalf("Failed to load config from %s: %v", path, err)
	}

	// 映射到原有全局变量
	DataBaseConfig = globalConfig.Postgres
	MongoDBConfig = globalConfig.MongoDB
	ServerConfig = globalConfig.Server
	RedisConfigInstance = globalConfig.Redis

	// 环境变量覆盖基础设施地址（config.yaml 提供 localhost 默认值，docker-compose 提供容器名）
	subEnv(&DataBaseConfig.Host, "POSTGRES_HOST")
	subEnv(&DataBaseConfig.Port, "POSTGRES_PORT")
	subEnv(&DataBaseConfig.User, "POSTGRES_USER")
	subEnv(&DataBaseConfig.Password, "POSTGRES_PASSWORD")
	subEnv(&DataBaseConfig.DBName, "POSTGRES_DB_NAME")
	subEnv(&MongoDBConfig.Host, "MONGODB_HOST")
	subEnv(&MongoDBConfig.Port, "MONGODB_PORT")
	subEnv(&MongoDBConfig.DBName, "MONGODB_DB_NAME")
	subEnv(&RedisConfigInstance.Host, "REDIS_HOST")
	subEnv(&RedisConfigInstance.Port, "REDIS_PORT")
	subEnv(&ServerConfig.Port, "SERVER_PORT")
	// Kafka brokers 从逗号分隔的环境变量读取
	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		globalConfig.Kafka.Brokers = strings.Split(brokers, ",")
	}	// Kafka 消费者 GroupID：每台机器需要独立 group_id，各自消费全量消息并做本地广播。
	// 优先用环境变量 CHAT_GROUP_ID，未设置则以 hostname 为后缀。
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
}

// subEnv 如果环境变量存在则覆盖指针指向的值
func subEnv(ptr *string, envKey string) {
	if v := os.Getenv(envKey); v != "" {
		*ptr = v
	}
}
