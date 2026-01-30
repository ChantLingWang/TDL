package handlers

import (
	"context"
	"encoding/json"
	"sync"

	consumer_model "orchestrator_service/kafka/consumer/model"
	"orchestrator_service/kafka/producer"
	"orchestrator_service/orchestrator/saga"
)

// SagaEventHandlerContext Saga事件处理器上下文，封装所有必要的参数
type SagaEventHandlerContext struct {
	Ctx           context.Context
	EventData     json.RawMessage // 原始事件数据 (businessEvent.Data)
	BusinessEvent *consumer_model.BusinessEvent // 完整的业务事件对象
	KafkaProducer *producer.KafkaProducer
	Sagas         map[string]*saga.Saga
	SagasMutex    *sync.RWMutex
}
