package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chat_service/app/api/routes"
	config "chat_service/app/config"
	"chat_service/app/infrastructure/kafka"
	kafkaServices "chat_service/app/infrastructure/kafka/services"
	"chat_service/app/services"

	sdk_kafka "infrastructure_sdk/kafka"

	"github.com/gin-gonic/gin"
)

// initMessageService 初始化 Kafka Producer。
func initMessageService() *sdk_kafka.KafkaConnection {
	kafkaConfig := config.KafkaConfigInstance
	conn, err := sdk_kafka.NewKafkaConnection(kafkaConfig.Brokers, kafkaConfig.Topic, "producer-connection")
	if err != nil {
		log.Fatalf("创建 Kafka Producer 连接失败: %v", err)
	}
	kafka.NewKafkaProducer(conn, kafkaConfig.Topic)
	log.Println("Kafka Producer 已初始化")
	return conn
}

// createApp 创建 Gin 应用，只保留 gateway 需要的路由。
func createApp() *gin.Engine {
	engine := gin.Default()

	router := routes.NewRouter()
	router.Engine = engine
	router.SetupRoutes()

	engine.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "chat gateway",
			"version": "v2.0.0",
			"health":  "/api/v1/health",
		})
	})

	return engine
}

func startServer() {
	app := createApp()
	server := &http.Server{
		Addr:         ":" + config.ServerConfig.Port,
		Handler:      app,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

func main() {
	gin.ForceConsoleColor()
	config.InitConfig("config.yaml")

	// gateway 只保留 Kafka 生产者：把用户消息投递给 ai_service。
	producerConn := initMessageService()
	defer producerConn.Close()

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// AI 回复只推给目标用户的本地 WS 连接，gateway 不落库。
	kafkaServices.RegisterAiReplyBroadcast(func(targetUserID string, message []byte) {
		services.GetWSHub().BroadcastToUser(targetUserID, message)
	})

	consumerRunner := kafka.NewConsumerRunner()
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

	go startServer()

	<-sigchan
	log.Println("接收到关闭信号，服务即将退出")
	cancel()
}
