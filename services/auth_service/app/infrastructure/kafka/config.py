"""
Kafka配置管理
"""
from typing import List
from pydantic_settings import BaseSettings


class KafkaSettings(BaseSettings):
    """Kafka配置类"""
    
    # 基本连接配置
    bootstrap_servers: List[str] = ["kafka:29092"]
    client_id: str = "auth-service"
    
    # 主题配置
    topic_user_registered: str = "user-registrations"
    topic_user_updated: str = "user-updates"
    topic_saga_events: str = "saga-events"
    topic_sync_user_fields: str = "sync_user_fields"
    
    # 性能配置
    request_timeout_ms: int = 30000
    retries: int = 3
    retry_backoff_ms: int = 100
    
    # 批量发送配置
    batch_size: int = 1000
    linger_ms: int = 1000
    compression_type: str = "gzip"
    
    # 可靠性配置
    enable_idempotence: bool = True
    acks: str = "all"
    
    class Config:
        env_prefix = "KAFKA_"
        case_sensitive = False


# 创建全局配置实例
kafka_settings = KafkaSettings()