from pydantic import BaseModel


class ChatHistoryMessage(BaseModel):
    """chat_service GET /api/v1/messages/history 返回的单条消息"""
    sender_id: str
    content: str
    timestamp: int | str
    message_id: str = ""
    message_type: str = "text"
