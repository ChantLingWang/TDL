from services.auth_service.app.database.redis_service import RedisClient

class RedisUserService:
    def __init__(self):
        self.redis_client = RedisClient()
        
    def set_code(self, key: str, value: str, ttl: int):
        self.redis_client.set_update_data(key, value, ttl)
        
    def get_code(self, key: str):
        return self.redis_client.get_data(key)
    
    def delete_code(self, key: str):
        self.redis_client.redis_client.delete(key)
    
    def test_connection(self):
        return self.redis_client.test_connection()