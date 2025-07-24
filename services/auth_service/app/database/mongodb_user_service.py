from datetime import datetime, timezone
from typing import Optional,Dict,List,Any
from bson import ObjectId                                   # MongoDB 的对象ID，用于文档的唯一标识
import bcrypt                                               # 密码哈希库，用于安全存储密码
from .mongodb_service import MongoDBServiceManager,db_manager
from pymongo.results import UpdateResult


class MongoDBUserService:
    """用户集合操作封装"""
    def __init__(self,db_manager):
        """
        初始化用户集合管理器
        """
        self.connection_name = "Users"
        self.db_manager = db_manager
        self.collection = self.db_manager.get_collection(self.connection_name)
        
        
    #三个基础方法：创建用户，根据用户id获取用户信息，根据用户id更新用户信息
    async def create_user(self,user_data: Dict[str,Any]) -> str:
        """
        创建用户
        """
        try:
            #获取现在时间
            current_time = datetime.now(timezone.utc)

            #这两个字段，如果data中没有，也会自动增加
            user_data.setdefault('created_at',current_time)    #创建用户的时间

            #异步插入数据到 MongoDB 集合的操作，将user_data异步insert_one（mongo标准插入方法）插入到集合中
            result = await self.collection.insert_one(user_data)

            return user_data
        except Exception as e:
            raise Exception("创建用户失败")


    async def get_user_by_id(self,user_id:str) -> Optional[Dict[str,Any]]:  #返回类型是字典，用户信息，也可以是None
        """
        根据用户id获取用户信息
        """
        try:
            user = await self.collection.find_one(
                {"_id":ObjectId(user_id)}       #使用user_id查询用户信息,将user_id转换为ObjectId类型
            )
            return user
        except Exception as e:
            raise Exception("获取用户信息失败")
        
        
    async def update_user(self,user_id:str,update_data:Dict[str,Any]) -> UpdateResult:
        """
        根据用户id更新用户信息
        """
        try:
            result = await self.collection.update_one(
                {"_id":ObjectId(user_id)},   #查询条件
                {"$set":update_data}         #更新数据
            )   
            return result
        except Exception as e:
            raise Exception("更新用户信息失败")
        
        
    #扩展方法
    async def get_user_by_email(self,email:str) -> Optional[Dict[str,Any]]:
        """
        根据用户邮箱获取用户信息
        """
        try:
            user = await self.collection.find_one({"email":email})
            return user
        except Exception as e:
            raise Exception("根据邮箱获取用户信息失败")
        
