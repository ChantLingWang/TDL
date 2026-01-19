"""
Kafka生产者服务
负责Kafka连接管理和基本操作
"""
import logging
from typing import Optional
from confluent_kafka import Producer

from .config import kafka_settings

logger = logging.getLogger(__name__)


class KafkaProducerService:
    """Kafka生产者服务类，负责连接管理"""
    
    def __init__(self):
        """初始化Kafka生产者"""
        self.producer: Optional[Producer] = None
        self.bootstrap_servers = kafka_settings.bootstrap_servers
        self._connect()
    
    def _connect(self) -> None:
        """连接到Kafka集群"""
        try:
            # 将列表转换为字符串，confluent-kafka需要逗号分隔的字符串
            bootstrap_servers_str = ','.join(self.bootstrap_servers)
            
            config = {
                'bootstrap.servers': bootstrap_servers_str,
                'client.id': kafka_settings.client_id,
                'message.max.bytes': 1000000,
                'queue.buffering.max.messages': 10000,
                'queue.buffering.max.ms': kafka_settings.linger_ms,
                'batch.num.messages': kafka_settings.batch_size,
                'delivery.timeout.ms': kafka_settings.request_timeout_ms,
                'request.timeout.ms': kafka_settings.request_timeout_ms,
                'retry.backoff.ms': kafka_settings.retry_backoff_ms,
                'message.send.max.retries': kafka_settings.retries,
                'enable.idempotence': kafka_settings.enable_idempotence,
                'compression.type': kafka_settings.compression_type,
                'acks': kafka_settings.acks
            }
            
            self.producer = Producer(config)
        except Exception as e:
            logger.error(f"连接Kafka失败: {e}")
            self.producer = None


    def close(self) -> None:
        """关闭Kafka生产者"""
        if self.producer:
            try:
                self.producer.flush(timeout=10)  # 确保所有消息都已发送
            except Exception as e:
                logger.error(f"关闭Kafka生产者时出错: {e}")
                raise


# 创建全局生产者实例
kafka_producer = KafkaProducerService()