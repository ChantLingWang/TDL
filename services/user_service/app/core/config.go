package core

// 数据库配置
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	TimeZone string
	
	// MongoDB连接池配置参数
	MaxPoolSize              int
	MinPoolSize              int
	MaxIdleTimeMS            int
	ServerSelectionTimeoutMS int
	ConnectTimeoutMS         int
	SocketTimeoutMS          int
	RetryWrites              bool
	RetryReads               bool
}

// 服务器配置
type ServerCfg struct {
	Port string
}

// Kafka配置
type KafkaConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

// PostgreSQL数据库配置
var DataBaseConfig = DBConfig{
	Host:     "127.0.0.1",
	Port:     "5432",
	User:     "lianlinghao",
	Password: "",
	DBName:   "User",
	SSLMode:  "disable",
	TimeZone: "Asia/Shanghai",
}

// MongoDB数据库配置
var MongoDBConfig = DBConfig{
	Host:                     "127.0.0.1",
	Port:                     "27017",
	DBName:                   "User_Database",
	SSLMode:                  "disable",
	TimeZone:                 "Asia/Shanghai",
	MaxPoolSize:              10,
	MinPoolSize:              1,
	MaxIdleTimeMS:            30000,
	ServerSelectionTimeoutMS: 5000,
	ConnectTimeoutMS:         10000,
	SocketTimeoutMS:          10000,
	RetryWrites:              true,
	RetryReads:               true,
}

// 服务器配置
var ServerConfig = ServerCfg{
	Port: "8080",
}

// Kafka配置
var KafkaConfigInstance = KafkaConfig{
	Brokers: []string{"localhost:9092"},
	Topic:   "user_registered_events",
	GroupID: "user_service_group",
}
