from datetime import datetime
from typing import Any

from pydantic import BaseModel


class CommonParams(BaseModel):
    """对应 Go 侧 kafka.CommonParams"""
    event_type: str
    event_name: str
    event_id: str
    timestamp: str
    execution_mode: str = ""


class BusinessEvent(BaseModel):
    """对应 Go 侧 kafka.BusinessEvent"""
    common_params: CommonParams
    data: Any


class PrivateChatData(BaseModel):
    """user.chat.private 事件的 data 载荷"""
    sender_id: str
    target_user_id: str
    content: str
    timestamp: int          # unix 毫秒
    message_id: str
    message_type: str = "text"


class ChatHistoryMessage(BaseModel):
    """chat_service GET /api/v1/messages/history 返回的单条消息"""
    sender_id: str
    content: str
    timestamp: int
    message_id: str = ""
    message_type: str = "text"


class ChatServiceHistoryResponse(BaseModel):
    messages: list[ChatHistoryMessage] = []
