from datetime import datetime, timezone, timedelta
from typing import Optional,Dict,Any
from bson import ObjectId                                   # MongoDB 的对象ID，用于文档的唯一标识                                          # 密码哈希库，用于安全存储密码
from pymongo.results import UpdateResult
from app.database.mongodb_service import db_manager


class MongoDBUserTokenService:
    """用户token集合操作封装"""
    def __init__(
        self,
        db_manager = db_manager,
    ):
       
        self.connection_name = "UserTokens"
        self.db_manager = db_manager
        self.collection = self.db_manager.get_collection(self.connection_name)
    
    async def create_user_token(self,user_token_data: Dict[str,Any]) -> str:
        """
        创建用户token
        """
        try:
            #获取现在时间并转换为字符串格式
            current_time = datetime.now(timezone.utc).isoformat()
            user_token_data.setdefault('created_at', current_time)    #创建用户的时间，存储为字符串
            
            #设置过期时间为1年
            expire_time = datetime.now(timezone.utc) + timedelta(days=365)
            user_token_data.setdefault('expire_at', expire_time)    #过期时间，存储为字符串

            #异步插入数据到 MongoDB 集合的操作，将user_data异步insert_one（mongo标准插入方法）插入到集合中
            result = await self.collection.insert_one(user_token_data)

            return str(result.inserted_id)
        except Exception as e:
            raise Exception("创建用户token失败")