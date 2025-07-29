from redis import Redis
from services.auth_service.app.core.config import settings

class RedisClient:
    def __init__(self):
        self.redis_client = Redis(
            host=settings.redis_host,
            port=settings.redis_port,
            db=settings.redis_db
        )
    
    def set_update_data(self, key: str, value: str, ttl: int):
        self.redis_client.set(key, value,ex=ttl)
        
    def get_data(self, key: str):
        if self.redis_client.exists(key):
            return self.redis_client.get(key)
        else:
            return None        
        