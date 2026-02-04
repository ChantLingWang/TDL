package mongodb

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	config "user_service/app/config"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 继承自mongo.Client go为组合形式
type MongoDBManager struct {
	client *mongo.Client
}

// 定义单例模式确保只有一个数据库实例
var (
	instance *MongoDBManager
	once     sync.Once
)

func GetMongoDBManager() *MongoDBManager {
	once.Do(func() {
		instance = &MongoDBManager{}
	})
	return instance
}

// Connect 连接到MongoDB数据库
func (manager *MongoDBManager) Connect() error {
	// 直接从配置文件获取MongoDB连接URI
	host := config.MongoDBConfig.Host
	port := config.MongoDBConfig.Port
	dbname := config.MongoDBConfig.DBName

	uri := fmt.Sprintf("mongodb://%s:%s/%s?sslmode=%s&timezone=%s",
		host, port, dbname, config.MongoDBConfig.SSLMode, config.MongoDBConfig.TimeZone)

	// 创建客户端选项对象，配置连接池参数
	clientOptions := options.Client().ApplyURI(uri)
	
	// 设置连接池配置参数
	clientOptions.SetMaxPoolSize(uint64(config.MongoDBConfig.MaxPoolSize))
	clientOptions.SetMinPoolSize(uint64(config.MongoDBConfig.MinPoolSize))
	clientOptions.SetConnectTimeout(time.Duration(config.MongoDBConfig.ConnectTimeoutMS) * time.Millisecond)
	clientOptions.SetSocketTimeout(time.Duration(config.MongoDBConfig.SocketTimeoutMS) * time.Millisecond)
	clientOptions.SetServerSelectionTimeout(time.Duration(config.MongoDBConfig.ServerSelectionTimeoutMS) * time.Millisecond)
	clientOptions.SetMaxConnIdleTime(time.Duration(config.MongoDBConfig.MaxIdleTimeMS) * time.Millisecond)

	// 连接到MongoDB，传入配置的客户端选项
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Printf("Failed to connect to MongoDB: %v", err)
		return err
	}

	// 检查连接
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Printf("Failed to ping MongoDB: %v", err)
		return err
	}

	manager.client = client
	return nil
}

// GetDatabase 获取数据库实例
func (manager *MongoDBManager) GetDatabase() *mongo.Database {
	if manager.client == nil {
		return nil
	}
	return manager.client.Database(config.MongoDBConfig.DBName)
}

// Close 关闭MongoDB连接
func (manager *MongoDBManager) Close() error {
	if manager.client != nil {
		return manager.client.Disconnect(context.Background())
	}
	return nil
}