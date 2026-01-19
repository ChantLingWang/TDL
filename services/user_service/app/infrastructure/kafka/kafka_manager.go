package kafka

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
)

// KafkaConnection Kafka连接层，只负责连接和关闭
type KafkaConnection struct {
	reader *kafkago.Reader
	writer *kafkago.Writer
}

// NewKafkaConnection 创建新的Kafka连接
func NewKafkaConnection(brokers []string, topic string, groupID string) *KafkaConnection {
	// 动态 GroupID 逻辑
	finalGroupID := groupID
	if finalGroupID == "" {
		// 生成唯一 ID，例如: user-service-node-UUID
		finalGroupID = fmt.Sprintf("user-service-node-%s", uuid.New().String())
	}

	// 读取配置
	readerConfig := kafkago.ReaderConfig{
		Brokers:        brokers,
		GroupID:        finalGroupID,
		Topic:          topic,
		MinBytes:       10e3,        // 10KB
		MaxBytes:       10e6,        // 10MB
		CommitInterval: time.Second, // 提交偏移量间隔
		StartOffset:    kafkago.FirstOffset,
		ReadBackoffMin: 100 * time.Millisecond,
		ReadBackoffMax: 500 * time.Millisecond,
	}

	// 写入配置
	writerConfig := kafkago.WriterConfig{
		Brokers:      brokers,
		Topic:        topic,
		BatchSize:    1, // 立即发送，不等待批量
		BatchTimeout: 0,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		RequiredAcks: int(kafkago.RequireAll),
		Async:        false,
	}

	reader := kafkago.NewReader(readerConfig)
	writer := kafkago.NewWriter(writerConfig)

	return &KafkaConnection{
		reader: reader,
		writer: writer,
	}
}

// Close 关闭Kafka连接
func (kc *KafkaConnection) Close() error {
	if kc.reader != nil {
		kc.reader.Close()
	}

	if kc.writer != nil {
		kc.writer.Close()
	}

	return nil
}
