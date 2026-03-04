package kafka

import (
	"fmt"
	"sync"

	sdk_kafka "infrastructure_sdk/kafka"
	"orchestrator_service/config"
	"orchestrator_service/kafka/producer"
)

// KafkaManager Kafka连接管理器
type KafkaManager struct {
	config     config.KafkaConfig
	connection *sdk_kafka.KafkaConnection
	producer   *producer.KafkaProducer
}

// 使用单例模式确保只有一个Kafka管理器实例
var (
	kafkaInstance *KafkaManager
	kafkaOnce     sync.Once
)

// GetKafkaManager 获取Kafka管理器实例
func GetKafkaManager() *KafkaManager {
	kafkaOnce.Do(func() {
		kafkaInstance = &KafkaManager{
			config: config.KafkaConfig{
				Brokers: config.GlobalConfig.Kafka.Brokers,
				Topic:   config.GlobalConfig.Kafka.Topic,
				GroupID: config.GlobalConfig.Kafka.GroupID,
			},
		}
	})
	return kafkaInstance
}

// Connect 连接到Kafka集群
func (km *KafkaManager) Connect() error {
	// 创建Kafka连接
	conn, err := sdk_kafka.NewKafkaConnection(
		km.config.Brokers,
		km.config.Topic,
		km.config.GroupID,
	)
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}

	km.connection = conn
	km.producer = producer.NewKafkaProducer(conn)
	return nil
}

// GetProducer 获取Kafka生产者
func (km *KafkaManager) GetProducer() *producer.KafkaProducer {
	return km.producer
}

// GetConnection 获取Kafka连接
func (km *KafkaManager) GetConnection() *sdk_kafka.KafkaConnection {
	return km.connection
}

// GetConfig 获取Kafka配置
func (km *KafkaManager) GetConfig() config.KafkaConfig {
	return km.config
}

// Close 关闭Kafka连接
func (km *KafkaManager) Close() error {
	if km.connection != nil {
		return km.connection.Close()
	}
	return nil
}
