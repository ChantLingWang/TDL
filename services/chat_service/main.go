package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chat_service/app/api/routes"
	config "chat_service/app/config"
	"chat_service/app/database/mongodb"
	"chat_service/app/database/pgsql"
	"chat_service/app/infrastructure/kafka"
	kafkaServices "chat_service/app/infrastructure/kafka/services"
	"chat_service/app/services"

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
	kafkaConfig := config.KafkaConfigInstance

	// 1. 创建 Kafka 连接（Producer 用）
	conn, err := sdk_kafka.NewKafkaConnection(kafkaConfig.Brokers, kafkaConfig.Topic, "producer-connection")
	if err != nil {
		log.Fatalf("创建 Kafka Producer 连接失败: %v", err)
	}

	// 2. 创建 Producer（会在内部设置为全局单例）
	kafka.NewKafkaProducer(conn, kafkaConfig.Topic)
	log.Println("Kafka Producer 已初始化")

	return conn
}

// createApp 创建应用实例
func createApp() *gin.Engine {
	// 初始化Gin引擎
	engine := gin.Default()

	// 获取用户路由实例
	userRouter := routes.NewRouter()
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
	if err := app.Run(":" + config.ServerConfig.Port); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

func main() {
	// 0. 初始化配置
	config.InitConfig("config.yaml")

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
	// 先注册群消息本地广播函数
	kafkaServices.RegisterGroupMessageLocalBroadcast(func(groupID string, message []byte) {
		// 从数据库获取群成员
		userGroupService := pgsql.NewUserGroupService(pgsql.GetDBManager())
		members, err := userGroupService.GetGroupMembers(groupID)
		if err != nil {
			log.Printf("获取群成员失败: group_id=%s, err=%v", groupID, err)
			return
		}

		// 推送给本地在线用户
		hub := services.GetWSHub()
		for _, userID := range members {
			hub.BroadcastToUser(userID, message)
		}
		log.Printf("群消息已推送给本地在线用户: group_id=%s, members=%d", groupID, len(members))
	})

	// 注册私聊消息本地广播函数
	kafkaServices.RegisterPrivateMessageLocalBroadcast(func(targetUserID string, message []byte) {
		hub := services.GetWSHub()
		hub.BroadcastToUser(targetUserID, message)
		log.Printf("私聊消息已推送给本地在线用户: to=%s", targetUserID)
	})

	consumerRunner := kafka.NewConsumerRunner(
		kafkaServices.HandleChatMessageEvent,
		kafkaServices.HandleBroadcastMessageEvent,
	)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := consumerRunner.Run(ctx); err != nil {
				log.Printf("Kafka consumer error: %v, restarting in 3s...", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
			}
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
