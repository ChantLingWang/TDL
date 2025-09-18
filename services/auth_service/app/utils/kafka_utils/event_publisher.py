"""
事件发布器 - 封装事件发布逻辑
"""
import logging
from app.models.events import (
    UserRegisteredEvent,
)
from app.utils.kafka_utils.kafka_producer import kafka_producer
from app.core.kafka_config import get_kafka_config

logger = logging.getLogger(__name__)

class EventPublisher:
    """事件发布器"""
    
    def __init__(self):
        self.config = get_kafka_config()
    
    def publish_user_registered_event(self, event: UserRegisteredEvent) -> None:
        """
        发布用户注册事件
        
        Args:
            event: 用户注册事件对象，包含所有必要字段
        """
        try:
            kafka_producer.produce_event(
                topic=self.config.KAFKA_TOPIC_AUTH_EVENTS,
                key=event.user_id,
                value=event.model_dump()
            )
            
            logger.info(f"用户注册事件发布成功: {event.user_id}")
            
        except Exception as e:
            logger.error(f"发布用户注册事件失败: {e}")
            raise


# 全局事件发布器实例
event_publisher = EventPublisher()
