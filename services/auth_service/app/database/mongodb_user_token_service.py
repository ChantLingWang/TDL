from datetime import datetime, timezone, timedelta
from typing import Optional,Dict,Any
from bson import ObjectId                                   # MongoDB 的对象ID，用于文档的唯一标识                                          # 密码哈希库，用于安全存储密码
from pymongo.results import UpdateResult
from app.database.mongodb_service import db_manager
from app.services.jwt_service import JWTUtils


class MongoDBUserTokenService:
    """用户token集合操作封装"""
    def __init__(
        self,
        db_manager = db_manager,
    ):
       
        self.connection_name = "UserTokens"
        self.db_manager = db_manager
        self.collection = self.db_manager.get_collection(self.connection_name)
    
    async def create_user_token(self,user_data: Dict[str,Any]) -> str:
        """
        创建用户token字段
        """
        try:
            user_id = user_data.get('user_id')
            user_email = user_data.get('email')
            
            #获取现在时间并转换为字符串格式
            current_time = datetime.now(timezone.utc)
            
            #生成refresh_token
            refresh_token = JWTUtils.create_refresh_token(user_data)
            
            #设置过期时间为1年
            expire_time = (datetime.now(timezone.utc) + timedelta(days=365))
            
            #只存储必要的字段，区分数据库保证安全
            token_data = {
                '_id': ObjectId(),  # MongoDB自动生成
                'user_id': ObjectId(user_id),  # 确保ObjectId类型
                'email': user_email,
                'refresh_token': refresh_token,
                'is_valid': True,
                'created_at': current_time,  # Date类型
                'expire_at': expire_time,  # Date类型
            }

            #异步插入数据到 MongoDB 集合
            await self.collection.insert_one(token_data)

            return refresh_token
        except Exception as e:
            raise Exception("创建用户token失败")
    
    
    async def update_user_refresh_token(self,user_id:str,refresh_token:str) -> UpdateResult:
        """
        更新用户refresh_token
        """
        try:
            result = await self.collection.update_one(
                {"user_id":user_id},   #查询条件
                {"$set":{"refresh_token":refresh_token}},   #更新操作
            )
            return result
        except Exception as e:
            raise Exception("更新用户refresh_token失败")
    