"""
Kafka生产者工具类 - 负责消息发布
"""
import json
import logging
from typing import Dict, Any, Optional
from confluent_kafka import Producer
from confluent_kafka.error import KafkaError

from app.core.kafka_config import get_kafka_config

logger = logging.getLogger(__name__)


class KafkaProducerManager:
    """Kafka生产者管理器"""
    
    def __init__(self):
        self.config = get_kafka_config()
        self.producer = self._create_producer()
    
    def _create_producer(self) -> Producer:
        """创建Kafka生产者实例"""
        producer_config = {
            # Kafka集群地址，格式: host1:port1,host2:port2,...
            'bootstrap.servers': self.config.KAFKA_BOOTSTRAP_SERVERS,
            
            # 消息确认机制: 
            #   '0' - 不等待确认（性能最高，可靠性最低）
            #   '1' - 等待leader确认（平衡）  
            #   'all' - 等待所有副本确认（可靠性最高）
            'acks': self.config.KAFKA_PRODUCER_ACKS,
            
            # 消息发送失败时的重试次数
            'retries': self.config.KAFKA_PRODUCER_RETRIES,
            
            # 每个连接允许的最大未确认请求数
            # 启用幂等性时建议设置为1-5，保证消息顺序
            'max.in.flight.requests.per.connection': self.config.KAFKA_PRODUCER_MAX_IN_FLIGHT,
            
            # 启用幂等性，确保Exactly-Once语义
            # 防止网络重试导致的消息重复
            'enable.idempotence': True,
        }
        return Producer(producer_config)
    
    def delivery_report(self, err: Optional[KafkaError], msg) -> None:
        """消息投递回调函数"""
        if err is not None:
            logger.error(f"消息投递失败: {err}")
        else:
            logger.info(f"消息投递成功: {msg.topic()} [{msg.partition()}] @ {msg.offset()}")
    
    def produce_event(self, topic: str, key: str, value: Dict[str, Any]) -> None:
        """发布事件消息"""
        try:
            # 序列化消息值
            serialized_value = json.dumps(value)
            
            # 发布消息
            self.producer.produce(
                topic=topic,
                key=key,
                value=serialized_value,
                callback=self.delivery_report
            )
            
            # 触发投递
            self.producer.poll(0)
            
        except Exception as e:
            logger.error(f"发布消息失败: {e}")
            raise
    
    def flush(self, timeout: float = 30.0) -> None:
        """刷新生产者，确保所有消息都被投递"""
        self.producer.flush(timeout)
    
    def close(self) -> None:
        """关闭生产者"""
        self.producer.flush()
        logger.info("Kafka生产者已关闭")


# 全局生产者实例
kafka_producer = KafkaProducerManager()
