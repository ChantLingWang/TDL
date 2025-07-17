from datetime import datetime
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
        self.collection_name = self.db_manager.get_collection(self.connection_name)