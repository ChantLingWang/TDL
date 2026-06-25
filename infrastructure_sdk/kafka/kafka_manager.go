package kafka

import (
	"fmt"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

// KafkaConnection Kafka连接层，只负责连接和关闭
type KafkaConnection struct {
	Reader *kafkago.Reader
	Writer *kafkago.Writer
}

// NewKafkaConnection 创建新的Kafka连接
func NewKafkaConnection(brokers []string, topic string, groupID string) (*KafkaConnection, error) {
	// 参数校验：GroupID 必须传入
	if groupID == "" {
		return nil, fmt.Errorf("kafka groupID is required")
	}

	// 读取配置
	readerConfig := kafkago.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topic,
		MinBytes:       10e3,        // 10KB
		MaxBytes:       10e6,        // 10MB
		CommitInterval: time.Second, // 提交偏移量间隔
		StartOffset:    kafkago.LastOffset,
		ReadBackoffMin: 100 * time.Millisecond,
		ReadBackoffMax: 500 * time.Millisecond,
	}

	// 写入配置
	writerConfig := kafkago.WriterConfig{
		Brokers:      brokers,
		// Topic is set per-message in producer.writeMessage, not on the Writer
		BatchSize:    1,     // 立即发送，不等待批量
		BatchTimeout: 0,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		RequiredAcks: int(kafkago.RequireAll),
		Async:        false,
	}

	reader := kafkago.NewReader(readerConfig)
	writer := kafkago.NewWriter(writerConfig)

	return &KafkaConnection{
		Reader: reader,
		Writer: writer,
	}, nil
}

// Close 关闭Kafka连接
func (kc *KafkaConnection) Close() error {
	if kc.Reader != nil {
		kc.Reader.Close()
	}

	if kc.Writer != nil {
		kc.Writer.Close()
	}

	return nil
}
