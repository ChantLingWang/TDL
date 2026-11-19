from pydantic import BaseModel


class ChatHistoryMessage(BaseModel):
    """存储服务历史接口返回的单条消息（存储服务尚未接入，当前历史为空）"""
    sender_id: str
    content: str
    timestamp: int | str
    message_id: str = ""
    message_type: str = "text"
