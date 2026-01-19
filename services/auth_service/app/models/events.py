"""
Kafka事件模型定义
"""
from datetime import datetime, timezone
from typing import Dict, Any
from pydantic import BaseModel, Field


class BaseEvent(BaseModel):
    """基础事件模型"""
    event_type: str = Field(..., description="事件类型")
    event_id: str = Field(..., description="事件唯一标识")
    timestamp: datetime = Field(default_factory=lambda: datetime.now(timezone.utc), description="事件时间戳")
    event_producer: str = Field(default="auth_service", description="事件生产者")


class UserRegisteredEvent(BaseEvent):
    """用户注册事件发起模型"""
    event_name: str = Field(default="sync_user_fields", description="调用的模版名")
    user_data: Dict[str, Dict[str, Any]] = Field(..., description="步骤数据字典，Key为步骤Topic，Value为该步骤的数据")
    execution_mode: str = Field(default="sequential", description="执行模式，sequential(串行)或parallel(并行)")
