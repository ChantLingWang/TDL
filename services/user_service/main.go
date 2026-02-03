package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	v1 "user_service/app/api/v1"
	"user_service/app/core"
	"user_service/app/database/mongodb"
	"user_service/app/database/pgsql"
	"user_service/app/infrastructure/kafka"
	"user_service/app/services"

	sdk_kafka "infrastructure_sdk/kafka"

	"github.com/gin-gonic/gin"
)

// initPostgreSQL 初始化PostgreSQL数据库连接
func initPostgreSQL() {
	dbManager := pgsql.GetDBManager()

	if err := dbManager.Connect(); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	// 注意：这里不要 close，因为是长连接
	// defer dbManager.Close()

	// 初始化数据库表结构
	if err := dbManager.Initialize(); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
}

// initMongoDB 初始化MongoDB数据库连接
func initMongoDB() {
	mongoManager := mongodb.GetMongoDBManager()
	if err := mongoManager.Connect(); err != nil {
		log.Fatalf("MongoDB连接失败: %v", err)
	}
}

// initMessageService 初始化消息服务（包括 Kafka Producer）
func initMessageService() *sdk_kafka.KafkaConnection {
	kafkaConfig := core.KafkaConfigInstance

	// 1. 创建 Kafka 连接（Producer 用）
	conn, err := sdk_kafka.NewKafkaConnection(kafkaConfig.Brokers, kafkaConfig.Topic, "producer-connection")
	if err != nil {
		log.Fatalf("创建 Kafka Producer 连接失败: %v", err)
	}

	// 2. 创建 Producer
	producer := kafka.NewKafkaProducer(conn, kafkaConfig.Topic)

	// 3. 初始化 MessageService
	services.InitMessageService(producer)
	log.Println("MessageService 已初始化")

	return conn
}

// createApp 创建应用实例
func createApp() *gin.Engine {
	// 初始化Gin引擎
	engine := gin.Default()

	// 获取用户路由实例
	userRouter := v1.NewRouter()
	userRouter.Engine = engine

	// 设置用户API路由
	userRouter.SetupRoutes()

	// 根路径
	engine.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello World",
			"version": "v1.0.0",
			"health":  "/api/v1/health",
		})
	})

	return engine
}

// startServer 启动HTTP服务器
func startServer() {
	// 创建应用实例
	app := createApp()

	// 启动HTTP服务器
	if err := app.Run(":8080"); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

func main() {
	// 1. 初始化数据库连接
	initPostgreSQL()
	initMongoDB()

	// 2. 初始化消息服务 (包含 Producer)
	producerConn := initMessageService()
	defer producerConn.Close()

	// 3. 设置信号处理
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// 创建上下文用于控制消费者生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. 启动 Kafka 消费者
	consumerRunner := kafka.NewConsumerRunner(
		services.HandleChatMessageEvent,
		services.HandleBroadcastMessageEvent,
	)

	go func() {
		if err := consumerRunner.Run(ctx); err != nil {
			log.Printf("Kafka consumer error: %v", err)

		}
	}()

	// 5. 启动 HTTP 服务器
	go startServer()

	// 6. 等待关闭信号
	<-sigchan
	log.Println("接收到关闭信号，服务即将退出")

	// 取消上下文，通知消费者停止
	cancel()
	// 可以添加一个短暂的等待让消费者清理资源，或者直接退出
}
