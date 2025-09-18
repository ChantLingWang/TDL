"""
Kafka健康检查
"""
import logging
from typing import Dict, Any
from confluent_kafka import Producer

from app.core.kafka_config import get_kafka_config

logger = logging.getLogger(__name__)


class KafkaHealthChecker:
    """Kafka健康检查器"""
    
    def __init__(self):
        self.config = get_kafka_config()
    
    async def check_health(self) -> Dict[str, Any]:
        """检查Kafka健康状态"""
        try:
            # 创建临时生产者测试连接
            producer = Producer({
                'bootstrap.servers': self.config.KAFKA_BOOTSTRAP_SERVERS,
                'socket.timeout.ms': 5000,  # 5秒超时
            })
            
            # 获取集群元数据来测试连接
            metadata = producer.list_topics(timeout=5.0)
            
            # 检查主题是否存在
            topics = [
                self.config.KAFKA_TOPIC_AUTH_EVENTS
            ]
            
            missing_topics = []
            for topic in topics:
                if topic not in metadata.topics:
                    missing_topics.append(topic)
            
            producer.flush(timeout=1.0)
            
            health_status = {
                'status': 'healthy' if not missing_topics else 'degraded',
                'bootstrap_servers': self.config.KAFKA_BOOTSTRAP_SERVERS,
                'topics_available': len(metadata.topics),
                'missing_topics': missing_topics,
                'error': None
            }
            
            if missing_topics:
                logger.warning(f"Kafka主题缺失: {missing_topics}")
            else:
                logger.info("Kafka连接健康检查通过")
            
            return health_status
            
        except Exception as e:
            logger.error(f"Kafka健康检查失败: {e}")
            return {
                'status': 'unhealthy',
                'bootstrap_servers': self.config.KAFKA_BOOTSTRAP_SERVERS,
                'topics_available': 0,
                'missing_topics': topics,
                'error': str(e)
            }


# 全局健康检查实例
kafka_health_checker = KafkaHealthChecker()