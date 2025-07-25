from redis import Redis
from app.core.config import settings

class RedisClient:
    def __init__(self):
        self.redis_client = Redis(
            host=settings.redis_host,
            port=settings.redis_port,
            db=settings.redis_db
        )
        
        