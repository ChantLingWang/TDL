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
        
        
    #基础方法：创建用户，根据用户id获取用户信息，根据用户id更新用户信息，根据用户id删除用户
    async def create_user(self,user_data: Dict[str,Any]) -> str:
        """
        创建用户
        """
        try:
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
            
    
    async def delete_user_by_id(self, user_id:str) -> UpdateResult:
        """
        根据用户id删除用户信息
        """
        try:
            result = await self.collection.delete_one(
                {"user_id":user_id},         #查询条件
            )   
            return result
        except Exception as e:
            raise Exception("删除用户信息失败")
        
        
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
    
    
    async def update_user_password_by_id(self, user_id:str, password:str) -> UpdateResult:
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
        获取自增序列值
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
            raise Exception("获取自增序列值失败")
    
    
    async def get_next_user_id(self) -> int:
        """
        获取下一个用户id（纯数字，从1开始递增）
        """
        try:
            return await self.get_next_value("user_id_sequence")
        except Exception as e:
            raise Exception("获取下一个用户id失败")
    

    async def sync_user_fields(self, user_data: Dict[str, Any]) -> None:
        """
        同步用户字段到数据库
        """
        try:
            await self.collection.insert_one(user_data)
        except Exception as e:
            raise Exception("同步用户字段失败")

    async def update_user_status(self, user_id: str, status: str) -> bool:
        """
        更新用户状态
        
        Args:
            user_id: 用户ID
            status: 新状态 (active/pending/error)
            
        Returns:
            bool: 是否更新成功
        """
        try:
            result = await self.collection.update_one(
                {"user_id": user_id},
                {"$set": {"status": status, "updated_at": datetime.now(timezone.utc)}}
            )
            return result.modified_count > 0
        except Exception as e:
            raise Exception(f"更新用户状态失败: {e}")

    async def get_user_status(self, user_id: str) -> Optional[str]:
        """
        获取用户状态
        
        Args:
            user_id: 用户ID
            
        Returns:
            str: 用户状态 (active/pending/error) 或 None
        """
        try:
            user = await self.collection.find_one(
                {"user_id": user_id},
                {"status": 1, "_id": 0}
            )
            return user.get("status") if user else None
        except Exception as e:
            raise Exception(f"获取用户状态失败: {e}")

    async def update_last_offline_time(self, user_id: str, timestamp: int) -> bool:
        """
        更新用户最后离线时间
        
        Args:
            user_id: 用户ID
            timestamp: 时间戳（秒）
            
        Returns:
            bool: 是否更新成功
        """
        try:
            result = await self.collection.update_one(
                {"user_id": user_id},
                {"$set": {"last_offline_time": timestamp}}
            )
            return result.modified_count > 0 or result.matched_count > 0
        except Exception as e:
            raise Exception(f"更新用户最后离线时间失败: {e}")

    async def get_last_offline_time(self, user_id: str) -> Optional[int]:
        """
        获取用户最后离线时间
        
        Args:
            user_id: 用户ID
            
        Returns:
            int: 时间戳（秒），如果不存在则返回 None
        """
        try:
            user = await self.collection.find_one(
                {"user_id": user_id},
                {"last_offline_time": 1, "_id": 0}
            )
            return user.get("last_offline_time") if user else None
        except Exception as e:
            raise Exception(f"获取用户最后离线时间失败: {e}")
