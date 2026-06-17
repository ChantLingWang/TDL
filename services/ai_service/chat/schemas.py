from pydantic import BaseModel


class ChatRequest(BaseModel):
    """从 Kafka PrivateChatData 解析出的对话请求"""
    user_id: str
    content: str
    message_id: str
    timestamp: int
