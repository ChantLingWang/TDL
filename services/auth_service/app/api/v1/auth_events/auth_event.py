"""
API层事件发布模块 - 提供统一的事件发布接口
"""
import logging
from typing import Dict, Any, Optional
from uuid import uuid4
from pydantic import ValidationError
from app.models.events import UserRegisteredEvent
from app.utils.kafka_utils.event_publisher import event_publisher

logger = logging.getLogger(__name__)

def publish_user_register_event(
    user_id: str,
    username: str,
    email: str,
    metadata: Optional[Dict[str, Any]] = None
) -> Dict[str, Any]:
    """
    发布用户注册事件 - API层统一入口
    
    Args:
        user_id: 用户唯一标识
        username: 用户名
        email: 用户邮箱
        metadata: 可选元数据
        
    Returns:
        标准化响应结果
    """

    # 参数验证由Pydantic模型自动处理，无需手动验证
    try:
        # 生成唯一事件ID（便于追踪）
        event_id = str(uuid4())

        # 这里不直接操作Kafka，而是通过专门的Publisher
        event = UserRegisteredEvent(
            event_id=event_id,
            user_id=user_id,
            username=username,
            email=email,
            event_producer="auth-service",
            payload={
                **(metadata or {})  # 合并用户提供的元数据
            }
        )
        
        event_publisher.publish_user_registered_event(event)
        
    except ValidationError as e:
        # Pydantic自动验证失败
        error_msg = f"参数验证失败: {e}"
        logger.error(error_msg)
        return {
            "success": False,
            "event_id": None,
            "error": str(e),
            "message": "failed"
        }
    
    
    # 返回标准化成功结果
    return {
        "success": True,
        "event_id": event_id,
        "message": "success",
        "details": {
            "user_id": user_id,
            "username": username,
            "email": email
        }
    }
