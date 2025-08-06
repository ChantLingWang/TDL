from datetime import datetime, timezone
from typing import Optional,Dict,Any
from bson import ObjectId                                   # MongoDB 的对象ID，用于文档的唯一标识                                          # 密码哈希库，用于安全存储密码
from pymongo.results import UpdateResult
from app.database.mongodb_service import db_manager


class MongoDBUserService:
    """用户集合操作封装"""
    def __init__(
        self,
        db_manager = db_manager
    ):
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
            #获取现在时间并转换为字符串格式
            current_time = datetime.now(timezone.utc).isoformat()

            #这个字段，如果data中没有，也会自动增加
            user_data.setdefault('created_at', current_time)    #创建用户的时间，存储为字符串

            #异步插入数据到 MongoDB 集合的操作，将user_data异步insert_one（mongo标准插入方法）插入到集合中
            result = await self.collection.insert_one(user_data)

            return str(result.inserted_id)
        except Exception as e:
            raise Exception("创建用户失败")


    async def get_user_by_id(self,user_id:str, fields: Optional[Dict[str, int]] = None) -> Optional[Dict[str,Any]]:
        """
        根据用户id获取用户信息
        
        Args:
            user_id: 用户ID
            fields: 字段投影配置，例如：{"_id": 0, "password": 0, "email": 1}
                   不传则使用默认配置（排除_id和password）
        """
        try:
            projection = fields or {"_id": 0, "password": 0}
            user = await self.collection.find_one(
                {"_id":ObjectId(user_id)},
                projection
            )
            return user
        except Exception as e:
            raise Exception("获取用户信息失败")
        
        
    async def update_user_by_id(self, user_id:str, update_data:Dict[str, Any]) -> UpdateResult:
        """
        根据用户id更新用户信息
        """
        try:
            result = await self.collection.update_one(
                {"user_id":user_id},         #查询条件
                {"$set":update_data}         #更新数据
            )   
            return result
        except Exception as e:
            raise Exception("更新用户信息失败")
    
    
    async def check_uid(self,user_id:str) -> bool:
        """
        检查用户id是否存在
        """
        try:
            user_id_status = await self.collection.find_one(
                {"user_id":str(user_id)},
                {"user_id": 1},
            )
            return user_id_status is not None
        except Exception as e:
            raise Exception("检查用户id失败")
        
        
    #扩展方法
    async def get_user_by_email(self,email:str, fields: Optional[Dict[str, int]] = None) -> Optional[Dict[str,Any]]:
        """
        根据用户邮箱获取用户信息
        
        Args:
            email: 用户邮箱
            fields: 字段投影配置，例如：{"_id": 0, "password": 0, "email": 1}
                   不传则使用默认配置（排除_id和password）
        """
        try:
            projection = fields or {"_id": 0, "password": 0}
            user = await self.collection.find_one(
                {"email":email},
                projection
            )
            return user
        except Exception as e:
            raise Exception("根据邮箱获取用户信息失败")
    
    async def get_user_by_email_with_password(self, email: str) -> Optional[Dict[str, Any]]:
        """
        根据用户邮箱获取用户信息（包含密码，用于密码验证）
        """
        try:
            user = await self.collection.find_one(
                {"email": email},
                {"_id": 0}  # 只排除_id，保留password用于验证
            )
            return user
        except Exception as e:
            raise Exception("根据邮箱获取用户信息失败")
    
    
    async def updata_user_password_by_id(self, user_id:str, password:str) -> UpdateResult:
        """
        根据用户id更新用户密码
        """
        try:
            result = await self.collection.update_one(
                {"user_id":user_id},   #查询条件
                {"$set":{"password":password}}         #更新数据
            )   
            return result
        except Exception as e:
            raise Exception("更新用户密码失败")
        
    async def get_next_value(self,sequence_name: str) -> int:
        """
        获取下一个序列值
        """
        try:
            result = await self.collection.find_one_and_update(
                {"_id": sequence_name},
                {"$inc": {"sequence_value": 1}},
                {"_id": 0, "sequence_value": 1},
                upsert=True
            )
            return result["sequence_value"]
        except Exception as e:
            raise Exception("获取下一个序列值失败")
    
    
    async def get_next_user_id(self) -> int:
        """
        获取下一个用户id（纯数字，从1开始递增）
        """
        try:
            return await self.get_next_value("user_id_sequence")
        except Exception as e:
            raise Exception("获取下一个用户id失败")