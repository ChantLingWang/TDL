package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orchestrator_service/kafka"
	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator"
	"orchestrator_service/templates"

	sdk_kafka "infrastructure_sdk/kafka"
)

func main() {

	// 初始化全局配置
	InitGlobalConfig()

	// 设置模板路径
	templates.SetTemplatePath(GlobalConfig.Templates.Path)

	// Kafka配置
	brokers := GlobalConfig.Kafka.Brokers
	topic := GlobalConfig.Kafka.Topic
	groupID := GlobalConfig.Kafka.GroupID

	// 1. 初始化 Kafka 生产者连接
	// 注意：这里需要一个单独的连接给 Producer 使用，因为 ConsumerRunner 会管理它自己的 Consumer 连接
	// 这符合读写分离的原则，也避免了连接复用带来的复杂性
	kafkaConn, err := sdk_kafka.NewKafkaConnection(brokers, topic, groupID)
	if err != nil {
		log.Fatalf("Failed to create Kafka connection for producer: %v", err)
	}
	defer kafkaConn.Close()

	kafkaProducer := producer.NewKafkaProducer(kafkaConn)

	// 2. 初始化带Kafka的编排器
	orchestratorInstance := orchestrator.NewSagaOrchestratorWithKafka(kafkaProducer)

	// 3. 设置信号处理和上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. 初始化并启动 Kafka 消费者 (ConsumerRunner)
	consumerConfig := kafka.KafkaConfig{
		Brokers: GlobalConfig.Kafka.Brokers,
		Topic:   GlobalConfig.Kafka.Topic,
		GroupID: GlobalConfig.Kafka.GroupID,
	}

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

	for {
		select {
		case <-ctx.Done():
			log.Println("Saga Executor stopped")
			return
		case <-orchestrator.GetContext().Done(): // 同时也监听 orchestrator 自身的上下文
			return
		case <-ticker.C:
			// 执行超时检查
			timeoutThreshold := 10 * time.Second
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
