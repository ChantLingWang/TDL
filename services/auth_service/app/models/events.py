"""
事件模型定义 - 用于Kafka消息传递
"""
from pydantic import BaseModel, Field
from datetime import datetime
from typing import Dict, Any
from enum import Enum

class EventType(str, Enum):
    """
    事件类型枚举
    创建不同的事件类型,就好比打上标签，这里只是定义了有什么事件
    """
    USER_REGISTERED = "user_registered"
    USER_LOGGED_IN = "user_logged_in"

class UserEvent(BaseModel):
    """
    用户事件模型
    定义了所有事件共同的字段
    """
    event_id: str = Field(..., description="事件唯一ID，UUID格式")
    event_type: EventType = Field(..., description="事件类型枚举值")
    event_time: datetime = Field(default_factory=datetime.now, description="事件发生时间，ISO格式")
    user_id: str = Field(..., description="用户唯一标识")
    version: str = Field(default="1.0.0", description="事件格式版本，用于schema演化")
    payload: Dict[str, Any] = Field(default_factory=dict, description="事件相关数据，JSON对象")
    event_producer: str = Field(..., description="事件生产者服务名称")

class UserRegisteredEvent(UserEvent):
    """用户注册事件"""
    event_type: EventType = EventType.USER_REGISTERED
    
    # 注册事件特定字段
    username: str = Field(..., description="用户名")
    email: str = Field(..., description="用户邮箱")
    user_id: str = Field(..., description="用户唯一标识")
