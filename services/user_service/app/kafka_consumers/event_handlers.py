"""
事件处理器 - 处理具体的业务逻辑
"""
import logging
from typing import Dict, Any
from app.database.mongodb_user_service import MongoDBUserService

logger = logging.getLogger(__name__)


class EventHandlers:
    """事件处理器类"""
    
    def __init__(self):
        self.user_service = MongoDBUserService()
    
    async def handle_user_registered_event(self, event_data: Dict[str, Any]) -> None:
        """处理用户注册事件"""
        try:
            user_id = event_data.get("user_id")
            username = event_data.get("username")
            email = event_data.get("email")
            
            if not all([user_id, username, email]):
                logger.error(f"用户注册数据不完整: {event_data}")
                return
            
            if await self.user_service.get_user_by_id(user_id):
                logger.warning(f"用户已存在: {user_id}")
                return
            # 构建用户的基础数据
            user_data = {
                "user_id": user_id,
                "username": username,
                "email": email,
                "source": "auth_service",
                "status": "active",
            }
            
            if await self.user_service.create_user(user_data):
                logger.info(f"用户注册成功: {user_id}")
            else:
                logger.error(f"用户注册失败: {user_id}")
                
        except Exception as e:
            logger.error(f"处理注册事件异常: {e}")
    
    def handle_user_logged_in_event(self, event_data: Dict[str, Any]) -> None:
        """处理用户登录事件"""
        try:
            user_id = event_data.get("user_id")
            login_time = event_data.get("login_time")
            logger.info(f"用户登录: {user_id}, time={login_time}")
        except Exception as e:
            logger.error(f"处理登录事件异常: {e}")
    
    def handle_unknown_event(self, event_data: Dict[str, Any]) -> None:
        """处理未知事件"""
        logger.warning(f"未知事件: {event_data}")


event_handlers = EventHandlers()