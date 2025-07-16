import asyncio
import ssl
from typing import Optional
from datetime import datetime
from motor.motor_asyncio import AsyncIOMotorClient, AsyncIOMotorDatabase
from pymongo.errors import ConnectionFailure
import logging

#配置日志
logging.basicConfig(level=logging.INFO)#这是python的日志配置，其中level代表日志提示等级，DEBUG：详细的调试信息；INFO：一般信息；WARNING：警告信息；ERROR：错误信息；CRITICAL：严重错误
logger = logging.getLogger(__name__)#创建日志记录器，使用logging.getLogger()方法，__name__表示为当前文件的name，如这里就是mongodb_service


class MongoDBServiceManager:
    """数据库连接管理类"""
    def __init__(
        self,
        connection_string:str="mongodb://localhost:27017",    #这里的链接应该为变量，在生产端时，要使用生产端数据库链接
        database_name:str="TDL_local_test_database",
        max_pool_size: int=10,
        min_pool_size: int=1,
        max_idle_time_ms: int=30000,
        server_selection_timeout_ms: int=5000,
        connect_timeout_ms: int=10000,
        socket_timeout_ms: int=10000,
        retry_writes: bool = True,
        retry_reads: bool = True,
        use_ssl: bool = False,
        ssl_cert_reqs: int = ssl.CERT_NONE,
    ):
        #细看代码，将我们定义的两个字段赋值给我们self属性
        self.connection_string = connection_string
        self.database_name = database_name

        #下面这段是连接状态的一些属性
        # 其实下面这句代码等于 self.client: Union[AsyncIOMotorClient, None] = None，意思是这个字段可以是AsyncIOMotorClient也可以是None，但默认是None
        self.client : Optional[AsyncIOMotorClient] = None  #默认值写在等号后，这部分要注意到optional这个关键字，是一个范围关键字，这个关键字表示这些属性可以是指定类型或者None
        self.database : Optional[AsyncIOMotorDatabase] = None
        self.is_connected: bool = False
        self.connection_timeout_ms: Optional[datetime] = None

        #这里是连接池的配置，python是动态语言，可以在构造函数直接定义属性，比如connection_config，这是个字典属性，由动态定义直接定义出来的
        self.connection_config = {
            'maxPoolSize': max_pool_size,
            'minPoolSize': min_pool_size,
            'maxIdleTimeMS': max_idle_time_ms,
            'serverSelectionTimeoutMS': server_selection_timeout_ms,
            'connectTimeoutMS': connect_timeout_ms,
            'socketTimeoutMS': socket_timeout_ms,
            'retryWrites': retry_writes,
            'retryReads': retry_reads
        }

        #这是ssl凭证配置，当ssl为True时，我们就利用update方法对connection_config这个字典属性进行扩展   ->  字典有很多功能强大的内置方法，在阅读代码时需要注意
        if use_ssl:
            self.connection_config.update({
                'ssl':True,
                'ssl_cert_reqs': ssl_cert_reqs
            })

        logger.info(f"MongoDB 连接管理器初始化完成 - 数据库: {database_name}")

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