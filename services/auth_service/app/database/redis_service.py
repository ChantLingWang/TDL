from redis import Redis, ConnectionPool
from services.auth_service.app.core.config_test import settings

# 创建全局连接池
redis_pool = ConnectionPool(
    host=settings.redis_host,
    port=settings.redis_port,
    db=settings.redis_db,
    max_connections=20,  # 设置最大连接数
    retry_on_timeout=True
)

class RedisClient:
    def __init__(self):
        # 使用全局连接池创建Redis客户端
        self.redis_client = Redis(connection_pool=redis_pool)
    
    def set_update_data(self, key: str, value: str, ttl: int):
        self.redis_client.set(key, value, ex=ttl)
    
    def get_data(self, key: str):
        if self.redis_client.exists(key):
            return self.redis_client.get(key)
        else:
            return None
    
    def test_connection(self):
        try:
            self.redis_client.ping()
            return True
        except:
            return False