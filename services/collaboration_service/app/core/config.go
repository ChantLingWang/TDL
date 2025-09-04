package core

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	Env          string
	MySQLHost    string
	MySQLPort    int
	MySQLDB      string
	MySQLUser    string
	MySQLPass    string
	MongoURI     string
	MongoDB      string
	RedisHost    string
	RedisPort    int
	RedisPass    string
	RedisDB      int
	KafkaBrokers string
	KafkaGroupID string
	JWTSecret    string
	JWTExpire    string
	GRPCHost     string
	GRPCPort     int
}

func LoadConfig() *Config {
	// 加载.env文件
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	return &Config{
		Port:         getEnv("PORT", "8080"),
		Env:          getEnv("ENV", "development"),
		MySQLHost:    getEnv("MYSQL_HOST", "localhost"),
		MySQLPort:    getEnvAsInt("MYSQL_PORT", 3306),
		MySQLDB:      getEnv("MYSQL_DATABASE", "collaboration_db"),
		MySQLUser:    getEnv("MYSQL_USERNAME", "root"),
		MySQLPass:    getEnv("MYSQL_PASSWORD", "password"),
		MongoURI:     getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:      getEnv("MONGO_DATABASE", "collaboration_db"),
		RedisHost:    getEnv("REDIS_HOST", "localhost"),
		RedisPort:    getEnvAsInt("REDIS_PORT", 6379),
		RedisPass:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:      getEnvAsInt("REDIS_DB", 0),
		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "collaboration-service"),
		JWTSecret:    getEnv("JWT_SECRET", "your-super-secret-jwt-key-here"),
		JWTExpire:    getEnv("JWT_EXPIRE", "24h"),
		GRPCHost:     getEnv("GRPC_HOST", "localhost"),
		GRPCPort:     getEnvAsInt("GRPC_PORT", 50051),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Invalid value for %s: %s, using default: %d", key, valueStr, defaultValue)
		return defaultValue
	}
	return value
}
