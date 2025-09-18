"""
事件服务 - 处理事件相关的业务逻辑
"""
import logging
from typing import Optional

from app.utils.event_publisher import event_publisher

logger = logging.getLogger(__name__)


class EventService:
    """事件服务"""
    
    async def handle_user_registration(
        self,
        user_id: str,
        username: str,
        email: str,
        metadata: Optional[dict] = None
    ) -> None:
        """处理用户注册事件"""
        try:
            event_publisher.publish_user_registered_event(
                user_id=user_id,
                username=username,
                email=email,
                metadata=metadata
            )
            logger.info(f"用户注册事件处理完成: {user_id}")
            
        except Exception as e:
            logger.error(f"处理用户注册事件失败: {e}")
            # 这里可以添加重试或补偿逻辑
            raise
    
    async def handle_user_login(
        self,
        user_id: str,
        login_method: str = "password",
        metadata: Optional[dict] = None
    ) -> None:
        """处理用户登录事件"""
        try:
            event_publisher.publish_user_logged_in_event(
                user_id=user_id,
                login_method=login_method,
                metadata=metadata
            )
            logger.info(f"用户登录事件处理完成: {user_id}")
            
        except Exception as e:
            logger.error(f"处理用户登录事件失败: {e}")
            raise
    
    async def handle_user_logout(
        self,
        user_id: str,
        logout_reason: Optional[str] = None,
        metadata: Optional[dict] = None
    ) -> None:
        """处理用户登出事件"""
        try:
            event_publisher.publish_user_logged_out_event(
                user_id=user_id,
                logout_reason=logout_reason,
                metadata=metadata
            )
            logger.info(f"用户登出事件处理完成: {user_id}")
            
        except Exception as e:
            logger.error(f"处理用户登出事件失败: {e}")
            raise
    
    async def handle_password_reset(
        self,
        user_id: str,
        reset_by: str = "user",
        metadata: Optional[dict] = None
    ) -> None:
        """处理密码重置事件"""
        try:
            event_publisher.publish_user_password_reset_event(
                user_id=user_id,
                reset_by=reset_by,
                metadata=metadata
            )
            logger.info(f"密码重置事件处理完成: {user_id}")
            
        except Exception as e:
            logger.error(f"处理密码重置事件失败: {e}")
            raise
    
    async def handle_token_refresh(
        self,
        user_id: str,
        token_type: str = "refresh_token",
        metadata: Optional[dict] = None
    ) -> None:
        """处理token刷新事件"""
        try:
            event_publisher.publish_user_token_refreshed_event(
                user_id=user_id,
                token_type=token_type,
                metadata=metadata
            )
            logger.info(f"token刷新事件处理完成: {user_id}")
            
        except Exception as e:
            logger.error(f"处理token刷新事件失败: {e}")
            raise


# 全局事件服务实例
event_service = EventService()