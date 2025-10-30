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

// 数据库配置
var DataBaseConfig = DBConfig{
	Host:     "localhost",
	Port:     "5432",
	User:     "chant",
	Password: "107827135",
	DBName:   "user_group",
	SSLMode:  "disable",
	TimeZone: "Asia/Shanghai",
}

// 服务器配置
var ServerConfig = ServerCfg{
	Port: "8080",
}
