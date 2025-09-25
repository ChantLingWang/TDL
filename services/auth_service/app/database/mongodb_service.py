import asyncio
import ssl
from typing import Optional
from datetime import datetime
from app.core.config_test import settings
from motor.motor_asyncio import AsyncIOMotorClient, AsyncIOMotorDatabase
from pymongo.errors import ConnectionFailure, ServerSelectionTimeoutError
import logging

#配置日志
logging.basicConfig(level=logging.INFO)#这是python的日志配置，其中level代表日志提示等级，DEBUG：详细的调试信息；INFO：一般信息；WARNING：警告信息；ERROR：错误信息；CRITICAL：严重错误
logger = logging.getLogger(__name__)#创建日志记录器，使用logging.getLogger()方法，__name__表示为当前文件的name，如这里就是mongodb_service



class MongoDBServiceManager:
    """数据库连接管理类"""
    def __init__(
        self,
        connection_string:str = None,    #这里的链接应该为变量，在生产端时，要使用生产端数据库链接
        database_name:str = None,          #初始化阶段，也就是构造函数，声明变量，明确初始化默认值和该属性类型
        
        #下面是连接池配置参数
        max_pool_size: int=10,
        min_pool_size: int=1,
        max_idle_time_ms: int=30000,
        server_selection_timeout_ms: int=5000,
        connect_timeout_ms: int=10000,
        socket_timeout_ms: int=10000,
        retry_writes: bool = True,
        retry_reads: bool = True,
        
        #下面是ssl配置参数
        use_ssl: bool = False,
        ssl_cert_reqs: int = ssl.CERT_NONE,
        ssl_certfile: str = None,                             # SSL 证书文件路径
        ssl_keyfile: str = None,                              # 私钥文件路径
        ssl_ca_certs: str = None,                             # CA 证书路径
    ):
        #细看代码，将我们定义的两个字段赋值给我们self属性
        # 如果参数为None，使用settings中的默认值
        self.connection_string = connection_string or settings.mongodb_url
        self.database_name = database_name or settings.database_name

        #下面这段是连接状态的一些属性
        # 其实下面这句代码等于 self.client: Union[AsyncIOMotorClient, None] = None，意思是这个字段可以是AsyncIOMotorClient也可以是None，但默认是None
        self.client : Optional[AsyncIOMotorClient] = None       #默认值写在等号后，这部分要注意到optional这个关键字，是一个范围关键字，这个关键字表示这些属性可以是指定类型或者None
        self.database : Optional[AsyncIOMotorDatabase] = None   #这里还有一些重要点，这里都是:连接，这是类型注解，告诉开发者这个类型检查器这个变量可以是 AsyncIOMotorClient 类型或 None
        self.is_connected: bool = False                         #后面的 = None是说明初始化默认值为None
        self.connect_time: Optional[datetime] = None
        self.ssl_certfile = ssl_certfile                        # 保存 SSL 证书路径
        self.ssl_keyfile = ssl_keyfile                          # 保存私钥路径
        self.ssl_ca_certs = ssl_ca_certs                        # 保存 CA 证书路径

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
                'ssl_cert_reqs': ssl_cert_reqs,
                'ssl_certfile': ssl_certfile,
                'ssl_keyfile': ssl_keyfile,
                'ssl_ca_certs': ssl_ca_certs
            })


        logger.info(f"MongoDB 连接管理器初始化完成 - 数据库: {database_name}")

    async def connect(self)-> bool:
        """建立 MongoDB 连接"""
        if self.is_connected:
            logger.info("MongoDB 连接已存在，跳过连接")
            return True

        #创建客户端实例
        #下面这段代码中，AsyncIOMotorClient这个函数除了接受我们定义的连接url参数，还接受其余我们定义的config参数，对其进行配置
        try:
            self.client = AsyncIOMotorClient(   #这一段直接使用了 = 号，就是直接使用AsyncIOMotorClient来实例化，实例名为client，
                self.connection_string,         #这里有一个非常重要的地方就是AsyncIOMotorClient(host='localhost', port=27017, **kwargs)，你能看到这个方法是异步数据库的示例类
                **self.connection_config,       #这个类有两个必有参数，这是个很重要的知识点，host='localhost', port=27017, **kwargs，后面这个**kwargs意味着除了这两个参数以外，还接受其余所有动态参数
            )

            #获取数据库实例
            self.database = self.client[self.database_name]

            # 这是 AsyncIOMotorClient 的内置方法，使用这个方法可以ping一下数据库
            await self.client.admin.command('ping')

            #所以这里就要更新连接状态，对这个属性进行更新
            self.is_connected = True
            self.connect_time = datetime.now()

            logger.info(f"MongoDB 连接成功 - 数据库: {self.database_name}")

            # 记录连接信息
            if self.client:
                server_info = await self.client.server_info()   #异步服务器函数，不能使用同步方法，因为这得到了一个future对象（没有实现的对象，可能跳出事件循环了在等待完成）
                logger.info(f"MongoDB 服务器版本: {server_info.get('version', 'unknown')}")
                logger.info(f"连接池配置: 最大连接数={self.connection_config['maxPoolSize']}, "f"最小连接数={self.connection_config['minPoolSize']}")
            return True

        #这一段是异常处理TODO:这里需要重点学习一下mongodb官方给出的异常类以及logger：日志记录器还有except和EXCEPTION的异常处理
        except (ConnectionFailure, ServerSelectionTimeoutError, Exception) as e:
            # 统一异常处理
            if isinstance(e, ConnectionFailure):
                error_type = "连接错误"
            elif isinstance(e, ServerSelectionTimeoutError):
                error_type = "服务器选择超时"
            else:
                error_type = "未知错误"

            error_msg = f"MongoDB 连接失败 - {error_type}: {str(e)}"
            logger.error(error_msg)

            # 直接清理资源
            try:
                if self.client:
                    self.client.close()
            except Exception as cleanup_error:
                logger.warning(f"关闭客户端时发生错误: {cleanup_error}")
            finally:
                # 重置连接状态
                self.client = None
                self.database = None
                self.is_connected = False
                self.connect_time = None

            raise ConnectionError(error_msg) from e


    #数据库的关闭逻辑
    async def close(self):
        if self.client:
            self.client.close()
            self.is_connected = False
            logger.info("MongoDB 数据库已关闭")

    #检测数据库是否连接
    async def test_connection(self):
        try:
            if not self.client:
                logger.error("数据库客户端为空")
                return False
            if not self.is_connected:
                logger.error("连接状态标志为False")
                return False
            await self.client.admin.command('ping')
            logger.info("数据库连接正常")
            return True
        except Exception as e:
            logger.error(f"数据库连接不正常: {str(e)}")
            return False

    def get_collection(self, collection_name: str):
        if self.database is None:
            raise Exception("数据库未连接")
        return self.database[collection_name]

#创建实例化数据库db_manager
db_manager = MongoDBServiceManager()

async def main():
    #异步的启动数据库连接
    await db_manager.connect()