package kafka

import (
	"context"
	"fmt"
	"log"
	"time"

	commonv1 "github.com/chant/chant/gen/go/chant/common/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// marshalOpts 序列化选项：使用 snake_case 字段名，保持与现有前端兼容。
var marshalOpts = protojson.MarshalOptions{
	EmitUnpopulated: false,
	UseProtoNames:   true,
}

// unmarshalOpts 反序列化选项：忽略未知字段。
var unmarshalOpts = protojson.UnmarshalOptions{
	DiscardUnknown: true,
}

// NewEventEnvelope 用 proto 消息创建一个事件信封。
// eventType 如 "chant.chat.v1.MessageSent"
// source 如 "chat-service"
func NewEventEnvelope(eventType, source string, msg proto.Message) (*commonv1.EventEnvelope, error) {
	data, err := marshalOpts.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal proto event: %w", err)
	}

	return &commonv1.EventEnvelope{
		EventId:   fmt.Sprintf("%s-%d", eventType, time.Now().UnixNano()),
		EventType: eventType,
		Source:    source,
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
	}, nil
}

// SendEnvelope 通过底层 KafkaProducer 发送事件信封。
func (kp *KafkaProducer) SendEnvelope(ctx context.Context, topic, key string, envelope *commonv1.EventEnvelope) error {
	msgBytes, err := marshalOpts.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	return kp.writeMessage(ctx, topic, key, msgBytes)
}

// ParseEnvelope 从 Kafka 消息 bytes 解析事件信封。
func ParseEnvelope(raw []byte) (*commonv1.EventEnvelope, error) {
	env := new(commonv1.EventEnvelope)
	if err := unmarshalOpts.Unmarshal(raw, env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return env, nil
}

// UnmarshalData 将信封的 data 字段反序列化为目标 proto 消息。
func UnmarshalData(env *commonv1.EventEnvelope, target proto.Message) error {
	return unmarshalOpts.Unmarshal(env.Data, target)
}

// ProtoHandlerFunc proto 版事件处理函数
type ProtoHandlerFunc func(ctx context.Context, envelope *commonv1.EventEnvelope) error

// StartProto 启动 proto 版消费循环。
// 不同于 Start（用 BusinessEvent），此方法用 ParseEnvelope 解析每条消息。
func (bc *BaseConsumer) StartProto(ctx context.Context, handler ProtoHandlerFunc) error {
	reader := bc.connection.Reader
	log.Printf("Kafka proto consumer started for topic: %s", reader.Config().Topic)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				log.Printf("Kafka proto fetch error: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			envelope, err := ParseEnvelope(msg.Value)
			if err != nil {
				log.Printf("ParseEnvelope error (offset %d): %v", msg.Offset, err)
				reader.CommitMessages(context.Background(), msg)
				continue
			}

			// 简易重试：最多 3 次
			const maxRetries = 3
			var handleErr error
			for i := 0; i < maxRetries; i++ {
				handleErr = handler(ctx, envelope)
				if handleErr == nil {
					break
				}
				log.Printf("Proto handler error (attempt %d): %v", i+1, handleErr)
				time.Sleep(time.Duration(1<<uint(i)) * time.Second)
			}

			if handleErr != nil {
				log.Printf("Failed to handle envelope %s after %d retries", envelope.EventId, maxRetries)
			}

			if err := reader.CommitMessages(context.Background(), msg); err != nil {
				log.Printf("Kafka proto commit error: %v", err)
			}
		}
	}
}
