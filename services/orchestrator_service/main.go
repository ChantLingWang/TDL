package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orchestrator_service/kafka"
	"orchestrator_service/kafka/consumer"
	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator"
	"orchestrator_service/templates"
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

	// 初始化Kafka连接和生产者
	kafkaConn := kafka.NewKafkaConnection(brokers, topic, groupID)
	kafkaProducer := producer.NewKafkaProducer(kafkaConn)

	// 初始化带Kafka的编排器
	orchestrator := orchestrator.NewSagaOrchestratorWithKafka(kafkaProducer)

	// 初始化事件消费者，传入编排器
	eventConsumer := consumer.NewBaseEventHandler(kafkaConn, orchestrator)

	// 启动Kafka消费者
	go func() {

		if err := eventConsumer.ConsumeEvents(context.Background()); err != nil {
			log.Printf("Event consumer error: %v", err)
		}
	}()

	// 启动Saga执行器（监听内部事件队列）
	go func() {

		startSagaExecutor(orchestrator)
	}()

	// 等待中断信号进行优雅关闭
	waitForShutdown(orchestrator)
}

// startSagaExecutor 启动Saga执行器
// 目前作为占位符，后续可用于执行超时检查、失败重试等后台任务
func startSagaExecutor(orchestrator *orchestrator.SagaOrchestratorWithKafka) {

	for {
		select {
		case <-orchestrator.GetContext().Done():

			return
		default:
			// 执行超时检查
			timeoutThreshold := 10 * time.Second
			orchestrator.CheckTimeouts(timeoutThreshold)
			
			time.Sleep(1 * time.Second)
		}
	}
}

// waitForShutdown 等待关闭信号
func waitForShutdown(orchestrator *orchestrator.SagaOrchestratorWithKafka) {

	// 创建信号通道
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	<-quit

	// 清理工作

	orchestrator.Shutdown()

	// 等待所有goroutine结束
	timeout := time.After(5 * time.Second)
	select {
	case <-orchestrator.GetContext().Done():

	case <-timeout:
		log.Println("Timeout waiting for graceful shutdown")
	}
}
