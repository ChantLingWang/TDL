package mongodb

import (
	"context"
	"fmt"
	"log"
	"sync"

	"chant/user_service/app/core"

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
	host := core.MongoDBConfig.Host
	port := core.MongoDBConfig.Port
	dbname := core.MongoDBConfig.DBName

	uri := fmt.Sprintf("mongodb://%s:%s/%s?sslmode=%s&timezone=%s",
		host, port, dbname, core.MongoDBConfig.SSLMode, core.MongoDBConfig.TimeZone)

	// 创建客户端选项对象，将uri传入后，配置客户端选项
	clientOptions := options.Client().ApplyURI(uri)

	// 连接到MongoDB，传入上面配置的客户端选项
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
	return manager.client.Database(core.MongoDBConfig.DBName)
}

// Close 关闭MongoDB连接
func (manager *MongoDBManager) Close() error {
	if manager.client != nil {
		return manager.client.Disconnect(context.Background())
	}
	return nil
}