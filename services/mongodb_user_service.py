from datetime import datetime, timezone
from typing import Optional,Dict,List,Any
from bson import ObjectId                                   # MongoDB 的对象ID，用于文档的唯一标识
import bcrypt                                               # 密码哈希库，用于安全存储密码
from services.mongodb_service import db_manager

class MongoDBUserService:
    """用户集合操作封装"""
    def __init__(self,db_manager):
        """
        初始化用户集合管理器
        """
        self.connection_name = "Users"
        self.db_manager = db_manager
        self.collection = self.db_manager.get_collection(self.connection_name)

    async def creat_user(self,user_data: Dict[str,Any]) -> str:
        """
        创建用户
        """
        try:
            #获取现在时间
            current_time = datetime.now(timezone.utc)

            #这两个字段，如果data中没有，也会自动增加
            user_data.setdefault('created_at',current_time)    #创建用户的时间
            user_data['updated_at'] = datetime.now(current_time)    #用户最后的更新时间

            #异步插入数据到 MongoDB 集合的操作，将user_data异步insert_one（mongo标准插入方法）插入到集合中
            result = await self.collection.insert_one(user_data)

            return str(result.inserted_id)
        except Exception as e:
            raise Exception("创建用户失败")
