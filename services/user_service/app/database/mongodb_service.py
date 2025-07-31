from motor.motor_asyncio import AsyncIOMotorClient

class MongoDBService:
    def __init__(self, settings: Settings):
        self.client = AsyncIOMotorClient(settings.mongodb_url)
        self.database = self.client[settings.database_name]
