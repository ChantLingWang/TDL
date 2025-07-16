import asyncio
from motor.motor_asyncio import AsyncIOMotorClient
from pymongo.errors import ConnectionFailure
import logging

#配置日志
logging.basicConfig(level=logging.INFO)#这是python的日志配置，其中level代表日志提示等级，DEBUG：详细的调试信息；INFO：一般信息；WARNING：警告信息；ERROR：错误信息；CRITICAL：严重错误
logger = logging.getLogger(__name__)#创建日志记录器，使用logging.getLogger()方法，__name__表示为当前文件的name，如这里就是mongodb_service


class MongoDBService:
    """数据库连接管理类"""
    def __init__(
        self,connection_string:str="mongodb://localhost:27017",    #这里的链接应该为变量，在生产端时，要使用生产端数据库链接
        database_name:str="TDL"
    ):
        self.connection_string = connection_string
        self.database_name = database_name
        self.client = None
        self.database = None
        self.is_connected = False

    async def connect(self):
        try:
            #创建异步客户端
            self.client = AsyncIOMotorClient(self.connection_string)
            #获取数据库
            self.database = self.client[self.database_name]
            #测试数据库连接
            await self.client.admin.command('ping')
            self.is_connected = True
            logger.info("连接成功")
        except ConnectionFailure as e:
            logger.error("连接失败")
            raise
        return self.is_connected


    async def get_collection(self, collection_name: str):
        if not self.is_connected:
            raise Exception("数据库未连接！请先调用 connect() 方法")

        return self.database[collection_name]


    async def close(self):
        if self.is_connected:
            await self.client.close()
            self.is_connected = False
            logger.info("数据库已关闭")
        return self.is_connected