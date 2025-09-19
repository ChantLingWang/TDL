"""
Kafka消费者管理器
"""
import json
import logging
from typing import Dict, Any, Callable
from confluent_kafka import Consumer, KafkaError
from app.core.kafka_config import get_kafka_config

logger = logging.getLogger(__name__)


class KafkaConsumerManager:
    """Kafka消费者管理器"""
    
    def __init__(self):
        self.config = get_kafka_config()
        self.consumer = self._create_consumer()
        self.running = False
        self.handlers = {}
        self.bootstrap_servers = self.config.KAFKA_BOOTSTRAP_SERVERS
        self.group_id = self.config.KAFKA_CONSUMER_GROUP_ID
    
    def _create_consumer(self) -> Consumer:
        """创建Kafka消费者"""
        consumer_config = {
            'bootstrap.servers': self.config.KAFKA_BOOTSTRAP_SERVERS,
            'group.id': self.config.KAFKA_CONSUMER_GROUP_ID,
            'auto.offset.reset': self.config.KAFKA_AUTO_OFFSET_RESET,
            'enable.auto.commit': False,
            'session.timeout.ms': self.config.KAFKA_SESSION_TIMEOUT_MS,
            'heartbeat.interval.ms': 3000,
            'max.poll.interval.ms': 300000,
            'max.poll.records': 500,
            'security.protocol': self.config.KAFKA_SECURITY_PROTOCOL,
            'sasl.mechanism': self.config.KAFKA_SASL_MECHANISM,
            'sasl.username': self.config.KAFKA_SASL_USERNAME,
            'sasl.password': self.config.KAFKA_SASL_PASSWORD,
        }
        return Consumer({k: v for k, v in consumer_config.items() if v})
    
    def register_handler(self, topic: str, handler: Callable[[Dict[str, Any]], None]) -> None:
        """注册主题处理器"""
        self.handlers[topic] = handler
    
    def subscribe(self, topics: list) -> None:
        """订阅主题"""
        self.consumer.subscribe(topics)
    
    def _process_message(self, message) -> bool:
        """处理单条消息"""
        try:
            # 获取消息主题
            topic = message.topic()
            
            # 查找对应的处理器
            handler = self.handlers.get(topic)
            if not handler:
                logger.warning(f"无处理器: {topic}")
                return False
            
            message_value = json.loads(message.value())
            handler(message_value)
            return True
        
        except json.JSONDecodeError as e:
            logger.error(f"JSON解析失败: {e}")
            return False
        
        except Exception as e:
            logger.error(f"处理失败: {e}")
            return False
    
    def start_consuming(self) -> None:
        """开始消费消息"""
        self.running = True
        while self.running:
            try:
                message = self.consumer.poll(1.0)
                if message is None:
                    continue
                
                if message.error():
                    if message.error().code() != KafkaError._PARTITION_EOF:
                        logger.error(f"Kafka错误: {message.error()}")
                    continue
                
                if self._process_message(message):
                    self.consumer.commit(message)
                    
            except KeyboardInterrupt:
                break
            except Exception as e:
                logger.error(f"消费异常: {e}")
        self.stop()
    
    def stop(self) -> None:
        """停止消费者"""
        self.running = False
        try:
            self.consumer.close()
        except Exception as e:
            logger.error(f"关闭失败: {e}")


kafka_consumer = KafkaConsumerManager()