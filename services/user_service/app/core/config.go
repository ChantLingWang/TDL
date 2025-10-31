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
}

// 服务器配置
type ServerCfg struct {
	Port string
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
	Host:     "127.0.0.1",
	Port:     "27017",
	DBName:   "User_Database",
	SSLMode:  "disable",
	TimeZone: "Asia/Shanghai",
}

// 服务器配置
var ServerConfig = ServerCfg{
	Port: "8080",
}
