"""
Kafka配置管理 - 云原生版本
"""
import os
import logging
from typing import Optional, List
from pydantic import BaseSettings
from functools import lru_cache

logger = logging.getLogger(__name__)


class KafkaConfig(BaseSettings):
    """Kafka配置类 - 云原生版本"""
    
    # Kafka服务器配置
    KAFKA_BOOTSTRAP_SERVERS: str = os.getenv(
        "KAFKA_BOOTSTRAP_SERVERS", 
        "localhost:9092"  # 本地Kafka服务器
    )
    
    # 安全配置
    KAFKA_SECURITY_PROTOCOL: str = os.getenv("KAFKA_SECURITY_PROTOCOL", "PLAINTEXT")
    KAFKA_SASL_MECHANISM: Optional[str] = os.getenv("KAFKA_SASL_MECHANISM")
    KAFKA_SASL_USERNAME: Optional[str] = os.getenv("KAFKA_SASL_USERNAME")
    KAFKA_SASL_PASSWORD: Optional[str] = os.getenv("KAFKA_SASL_PASSWORD")
    
    # 消费者配置
    KAFKA_CONSUMER_GROUP_ID: str = os.getenv(
        "KAFKA_CONSUMER_GROUP_ID", 
        "user_service_consumers"
    )
    
    # 消费者行为配置
    KAFKA_AUTO_OFFSET_RESET: str = os.getenv("KAFKA_AUTO_OFFSET_RESET", "earliest")
    KAFKA_ENABLE_AUTO_COMMIT: bool = os.getenv("KAFKA_ENABLE_AUTO_COMMIT", "true").lower() == "true"
    KAFKA_AUTO_COMMIT_INTERVAL_MS: int = int(os.getenv("KAFKA_AUTO_COMMIT_INTERVAL_MS", "5000"))
    KAFKA_SESSION_TIMEOUT_MS: int = int(os.getenv("KAFKA_SESSION_TIMEOUT_MS", "10000"))
    
    # 性能调优配置
    KAFKA_MAX_POLL_RECORDS: int = int(os.getenv("KAFKA_MAX_POLL_RECORDS", "500"))
    KAFKA_MAX_POLL_INTERVAL_MS: int = int(os.getenv("KAFKA_MAX_POLL_INTERVAL_MS", "300000"))
    KAFKA_FETCH_MIN_BYTES: int = int(os.getenv("KAFKA_FETCH_MIN_BYTES", "1"))
    KAFKA_FETCH_MAX_BYTES: int = int(os.getenv("KAFKA_FETCH_MAX_BYTES", "52428800"))
    KAFKA_FETCH_MAX_WAIT_MS: int = int(os.getenv("KAFKA_FETCH_MAX_WAIT_MS", "500"))
    
    # 主题配置
    KAFKA_TOPIC_AUTH_EVENTS: str = os.getenv("KAFKA_TOPIC_AUTH_EVENTS", "user_events")
    KAFKA_TOPIC_USER_EVENTS: str = os.getenv("KAFKA_TOPIC_USER_EVENTS", "user_events")
    
    # 重试配置
    KAFKA_RETRY_BACKOFF_MS: int = int(os.getenv("KAFKA_RETRY_BACKOFF_MS", "100"))
    KAFKA_RETRIES: int = int(os.getenv("KAFKA_RETRIES", "5"))
    
    # 连接配置
    KAFKA_REQUEST_TIMEOUT_MS: int = int(os.getenv("KAFKA_REQUEST_TIMEOUT_MS", "30000"))
    KAFKA_RECONNECT_BACKOFF_MS: int = int(os.getenv("KAFKA_RECONNECT_BACKOFF_MS", "100"))
    KAFKA_RECONNECT_BACKOFF_MAX_MS: int = int(os.getenv("KAFKA_RECONNECT_BACKOFF_MAX_MS", "1000"))
    
    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"


@lru_cache()
def get_kafka_config() -> KafkaConfig:
    """
    获取Kafka配置实例（带缓存）
    
    Returns:
        KafkaConfig: Kafka配置实例
    """
    return KafkaConfig()


def get_kafka_servers() -> List[str]:
    """
    获取Kafka服务器地址列表
    
    Returns:
        List[str]: Kafka服务器地址列表
    """
    config = get_kafka_config()
    return [server.strip() for server in config.KAFKA_BOOTSTRAP_SERVERS.split(",")]


def get_kafka_consumer_config() -> dict:
    """
    获取Kafka消费者配置
    
    Returns:
        dict: Kafka消费者配置字典
    """
    config = get_kafka_config()
    
    consumer_config = {
        "bootstrap.servers": config.KAFKA_BOOTSTRAP_SERVERS,
        "group.id": config.KAFKA_CONSUMER_GROUP_ID,
        "auto.offset.reset": config.KAFKA_AUTO_OFFSET_RESET,
        "enable.auto.commit": config.KAFKA_ENABLE_AUTO_COMMIT,
        "auto.commit.interval.ms": config.KAFKA_AUTO_COMMIT_INTERVAL_MS,
        "session.timeout.ms": config.KAFKA_SESSION_TIMEOUT_MS,
        "max.poll.records": config.KAFKA_MAX_POLL_RECORDS,
        "max.poll.interval.ms": config.KAFKA_MAX_POLL_INTERVAL_MS,
        "fetch.min.bytes": config.KAFKA_FETCH_MIN_BYTES,
        "fetch.max.bytes": config.KAFKA_FETCH_MAX_BYTES,
        "fetch.max.wait.ms": config.KAFKA_FETCH_MAX_WAIT_MS,
        "retry.backoff.ms": config.KAFKA_RETRY_BACKOFF_MS,
        "retries": config.KAFKA_RETRIES,
        "request.timeout.ms": config.KAFKA_REQUEST_TIMEOUT_MS,
        "reconnect.backoff.ms": config.KAFKA_RECONNECT_BACKOFF_MS,
        "reconnect.backoff.max.ms": config.KAFKA_RECONNECT_BACKOFF_MAX_MS,
    }
    
    # 安全配置
    if config.KAFKA_SECURITY_PROTOCOL != "PLAINTEXT":
        consumer_config.update({
            "security.protocol": config.KAFKA_SECURITY_PROTOCOL,
            "sasl.mechanism": config.KAFKA_SASL_MECHANISM,
            "sasl.username": config.KAFKA_SASL_USERNAME,
            "sasl.password": config.KAFKA_SASL_PASSWORD,
        })
    
    return consumer_config


def get_kafka_producer_config() -> dict:
    """
    获取Kafka生产者配置
    
    Returns:
        dict: Kafka生产者配置字典
    """
    config = get_kafka_config()
    
    producer_config = {
        "bootstrap.servers": config.KAFKA_BOOTSTRAP_SERVERS,
        "retries": config.KAFKA_RETRIES,
        "retry.backoff.ms": config.KAFKA_RETRY_BACKOFF_MS,
        "request.timeout.ms": config.KAFKA_REQUEST_TIMEOUT_MS,
        "reconnect.backoff.ms": config.KAFKA_RECONNECT_BACKOFF_MS,
        "reconnect.backoff.max.ms": config.KAFKA_RECONNECT_BACKOFF_MAX_MS,
    }
    
    # 安全配置
    if config.KAFKA_SECURITY_PROTOCOL != "PLAINTEXT":
        producer_config.update({
            "security.protocol": config.KAFKA_SECURITY_PROTOCOL,
            "sasl.mechanism": config.KAFKA_SASL_MECHANISM,
            "sasl.username": config.KAFKA_SASL_USERNAME,
            "sasl.password": config.KAFKA_SASL_PASSWORD,
        })
    
    return producer_config