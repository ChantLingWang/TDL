"""Kafka 生产者模块。

负责以 AI 用户身份构造 BusinessEvent 并发送到 Kafka。
chat_service 消费后自动存入 MongoDB、推送给前端用户。
"""

import json
import logging
from datetime import datetime, timezone

from aiokafka import AIOKafkaProducer

from config.settings import settings
from shared.models import BusinessEvent, CommonParams, PrivateChatData

logger = logging.getLogger(__name__)


async def create_producer() -> AIOKafkaProducer:
    """创建并启动 aiokafka 生产者。"""
    producer = AIOKafkaProducer(
        bootstrap_servers=settings.kafka_brokers,
        # 统一用 JSON 序列化
        value_serializer=lambda v: json.dumps(v).encode("utf-8"),
    )
    await producer.start()
    logger.info("kafka 生产者已启动")
    return producer


async def send_ai_reply(
    producer: AIOKafkaProducer,
    user_id: str,
    content: str,
    message_id: str,
) -> None:
    """以 AI 用户身份发送一条私聊回复。

    生成的 BusinessEvent 格式完全对齐 Go 侧 chat_service 的期望：
        common_params.event_type = "user.chat.private"
        data.sender_id = ai_user_id
        data.target_user_id = 原用户 ID

    chat_service 消费这条消息后：
        1. 存入 MongoDB private_message_history
        2. 通过 WS 推送给 target 用户
    """
    now_ms = int(datetime.now(timezone.utc).timestamp() * 1000)

    # 构造 data 载荷
    data = PrivateChatData(
        sender_id=settings.ai_user_id,
        target_user_id=user_id,
        content=content,
        timestamp=now_ms,
        message_id=message_id,
    )

    # 构造 Go 侧 kafka.BusinessEvent 等价结构
    event = BusinessEvent(
        common_params=CommonParams(
            event_type="user.chat.private",
            event_name="user.chat.private",
            event_id=message_id,
            timestamp=datetime.now(timezone.utc).isoformat(),
        ),
        data=data.model_dump(),
    )

    # 以 user_id 作为 Kafka key，保证同一用户的消息有序
    await producer.send(
        topic=settings.kafka_topic,
        key=user_id.encode(),
        value=event.model_dump(),
    )
    logger.info("AI 回复已发送  to=%s msg_id=%s", user_id, message_id)


async def send_error_reply(
    producer: AIOKafkaProducer,
    user_id: str,
    message_id: str,
) -> None:
    """LLM 调用失败时，发送一条友好的错误提示给用户。

    message_id 加 err- 前缀，避免和正常消息 ID 冲突。
    """
    await send_ai_reply(
        producer,
        user_id,
        "[AI 暂时无法回复，请稍后重试]",
        f"err-{message_id}",
    )
