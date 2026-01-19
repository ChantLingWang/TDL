"""
Kafka基础设施模块
提供Kafka生产者、消费者和事件处理功能
"""
from .kafka_manager import KafkaProducerService
from .event_publisher import EventPublisher
from ...models.events import BaseEvent, UserRegisteredEvent

__all__ = [
    'KafkaProducerService',
    'EventPublisher',
    'BaseEvent',
    'UserRegisteredEvent'
]