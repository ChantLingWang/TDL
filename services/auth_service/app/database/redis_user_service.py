from redis_service import RedisClient

class RedisUserService:
    def __init__(self):
        self.redis_client = RedisClient()
        
    def set_code(self, key: str, ttl: int):
        self.redis_client.set_update_data(key, ttl)
        