"""
Kafka配置管理
"""
import os
from pydantic_settings import BaseSettings
from functools import lru_cache


class KafkaConfig(BaseSettings):
    """Kafka配置类"""
    
    # Kafka连接配置
    KAFKA_BOOTSTRAP_SERVERS: str = "localhost:8092"
    """Kafka集群地址，格式: host1:port1,host2:port2,..."""
    
    # 安全认证配置
    KAFKA_SECURITY_PROTOCOL: str = "PLAINTEXT"
    """安全协议: PLAINTEXT(无加密), SSL(TLS加密), SASL_PLAINTEXT(SASL无加密), SASL_SSL(SASL+TLS)"""
    KAFKA_SASL_MECHANISM: str = "PLAIN"
    """SASL认证机制: PLAIN(用户名密码), SCRAM-SHA-256, SCRAM-SHA-512, GSSAPI(Kerberos)"""
    KAFKA_SASL_USERNAME: str = ""
    """SASL用户名"""
    KAFKA_SASL_PASSWORD: str = ""
    """SASL密码"""
    KAFKA_SSL_CA_LOCATION: str = ""
    """SSL CA证书文件路径，用于验证broker证书"""
    KAFKA_SSL_CERTIFICATE_LOCATION: str = ""
    """SSL客户端证书文件路径"""
    KAFKA_SSL_KEY_LOCATION: str = ""
    """SSL客户端私钥文件路径"""
    
    # 主题配置
    KAFKA_TOPIC_AUTH_EVENTS: str = "auth-events"
    """认证事件主题名称"""
    
    # 生产者配置
    KAFKA_PRODUCER_ACKS: str = "all"
    """消息确认机制: 0(不等待), 1(leader确认), all(所有副本确认)"""
    KAFKA_PRODUCER_RETRIES: int = 3
    """消息发送失败重试次数"""
    KAFKA_PRODUCER_MAX_IN_FLIGHT: int = 3
    """每个连接允许的最大未确认请求数，启用幂等性时建议设置为1-5"""
    KAFKA_PRODUCER_ENABLE_IDEMPOTENCE: bool = True
    """是否启用幂等性，确保消息Exactly-Once语义"""
    
    # 批处理和压缩
    KAFKA_PRODUCER_BATCH_SIZE: int = 16384
    """批处理大小(字节)，达到此大小或linger.ms时间后发送"""
    KAFKA_PRODUCER_LINGER_MS: int = 5
    """消息在发送前等待的毫秒数，用于批处理优化"""
    KAFKA_PRODUCER_COMPRESSION_TYPE: str = "none"
    """消息压缩类型: none, gzip, snappy, lz4, zstd"""
    KAFKA_PRODUCER_BUFFER_MEMORY: int = 33554432
    """生产者缓冲区总内存(字节)"""
    
    # 连接和超时配置
    KAFKA_SOCKET_TIMEOUT_MS: int = 30000
    """网络socket超时时间(毫秒)"""
    KAFKA_SESSION_TIMEOUT_MS: int = 10000
    """会话超时时间(毫秒)，用于检测消费者故障"""
    KAFKA_METADATA_MAX_AGE_MS: int = 300000
    """元数据缓存最大年龄(毫秒)，定期刷新集群信息"""
    
    # 监控和统计
    KAFKA_STATISTICS_INTERVAL_MS: int = 0
    """统计信息收集间隔(毫秒)，0表示禁用"""
    
    # 事务配置
    KAFKA_TRANSACTIONAL_ID: str = "auth-service-transactional"
    """事务ID，用于标识生产者事务"""
    KAFKA_TRANSACTION_TIMEOUT_MS: int = 60000
    """事务超时时间(毫秒)"""
    
    # 消费者配置
    KAFKA_CONSUMER_GROUP_ID: str = "auth-service-group"
    """消费者组ID，用于协调分区消费"""
    KAFKA_CONSUMER_AUTO_OFFSET_RESET: str = "earliest"
    """偏移量重置策略: earliest(最早), latest(最新), none(无偏移量时报错)"""
    
    # Schema Registry配置
    KAFKA_SCHEMA_REGISTRY_URL: str = "http://localhost:8081"
    """Schema Registry服务地址，用于Avro schema管理"""
    
    def __init__(self, **kwargs):
        super().__init__(**kwargs)
        # 根据环境设置不同的默认值
        self._set_environment_defaults()
    
    def _set_environment_defaults(self):
        """根据环境设置不同的默认配置"""
        if self.ENVIRONMENT == "production":
            # 生产环境配置
            self.KAFKA_BOOTSTRAP_SERVERS = "kafka-prod-1:9092,kafka-prod-2:9092"
            self.KAFKA_SECURITY_PROTOCOL = "SASL_SSL"
            self.KAFKA_PRODUCER_COMPRESSION_TYPE = "gzip"
            self.KAFKA_PRODUCER_LINGER_MS = 20
        elif self.ENVIRONMENT == "staging":
            # 预发布环境配置
            self.KAFKA_BOOTSTRAP_SERVERS = "kafka-staging:9092"
            self.KAFKA_SECURITY_PROTOCOL = "SASL_SSL"
    
    class Config:
        env_file = ".env"
        case_sensitive = False


@lru_cache()
def get_kafka_config() -> KafkaConfig:
    """获取Kafka配置实例"""
    return KafkaConfig()
