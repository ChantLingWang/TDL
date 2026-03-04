package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orchestrator_service/database/pgsql"
	"orchestrator_service/kafka"
	"orchestrator_service/kafka/handlers"
	"orchestrator_service/orchestrator"
	"orchestrator_service/templates"

	config "orchestrator_service/config"
)

func main() {

	// 初始化全局配置
	config.InitGlobalConfig("config/config.yaml")

	// 初始化雪花算法生成器
	if err := handlers.InitSnowflake(config.GlobalConfig.SnowflakeNodeID); err != nil {
		log.Fatalf("Failed to initialize snowflake node: %v", err)
	}

	// 设置模板路径
	templates.SetTemplatePath(config.GlobalConfig.Templates.Path)

	// 初始化数据库连接
	dbManager := pgsql.GetDBManager()

	// 设置 GEN 的默认数据库连接
	if err := dbManager.Connect(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	// 设置数据库的自动关闭
	defer dbManager.Close()

	// 自动创建数据库表结构
	if err := dbManager.Initialize(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 创建 Saga 持久化仓库
	sagaRepo := pgsql.NewPgsqlSagaRepository()

	// 初始化Kafka连接管理器
	kafkaManager := kafka.GetKafkaManager()
	if err := kafkaManager.Connect(); err != nil {
		log.Fatalf("Failed to connect to Kafka: %v", err)
	}
	defer kafkaManager.Close()

	// 获取Kafka生产者
	kafkaProducer := kafkaManager.GetProducer()

	// 初始化带Kafka的编排器
	orchestratorInstance := orchestrator.NewSagaOrchestratorWithKafka(kafkaProducer, sagaRepo)

	// 设置信号处理和上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化并启动Kafka消费者 (ConsumerRunner)
	consumerConfig := kafkaManager.GetConfig()
	consumerRunner := kafka.NewConsumerRunner(consumerConfig, orchestratorInstance)

	go func() {
		if err := consumerRunner.Run(ctx); err != nil {
			log.Printf("Kafka consumer runner error: %v", err)
		}
	}()

	// 5. 启动 Saga 执行器（后台任务，如超时检查）
	go func() {
		startSagaExecutor(ctx, orchestratorInstance)
	}()

	// 6. 等待关闭信号
	waitForShutdown(cancel, orchestratorInstance)
}

// startSagaExecutor 启动Saga执行器
// 目前作为占位符，后续可用于执行超时检查、失败重试等后台任务
func startSagaExecutor(ctx context.Context, orchestrator *orchestrator.SagaOrchestratorWithKafka) {
	log.Println("Starting Saga Executor...")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 使用配置中的超时值，如果未配置则使用默认值
	timeoutThreshold := config.GlobalConfig.Saga.ExecutionTimeout
	if timeoutThreshold == 0 {
		timeoutThreshold = 5 * time.Minute // 默认5分钟
	}
	log.Printf("Saga execution timeout threshold: %v", timeoutThreshold)

	for {
		select {
		case <-ctx.Done():
			log.Println("Saga Executor stopped")
			return
		case <-orchestrator.GetContext().Done(): // 同时也监听 orchestrator 自身的上下文
			return
		case <-ticker.C:
			// 执行超时检查
			orchestrator.CheckTimeouts(timeoutThreshold)
		}
	}
}

// waitForShutdown 等待关闭信号
func waitForShutdown(cancel context.CancelFunc, orchestrator *orchestrator.SagaOrchestratorWithKafka) {
	// 创建信号通道
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	<-quit

	// 1. 首先取消全局上下文，通知所有 Runner 停止
	cancel()

	// 2. 清理编排器资源
	orchestrator.Shutdown()

	// 3. 给一点时间让 goroutine 退出
	time.Sleep(2 * time.Second)
	log.Println("Orchestrator service exited")
}
