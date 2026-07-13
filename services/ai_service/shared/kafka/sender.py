"""Kafka 生产者模块。"""

import json
import logging
from datetime import datetime, timezone
from aiokafka import AIOKafkaProducer
from config.settings import settings
from shared.models import BusinessEvent, CommonParams, PrivateChatData, GroupChatData

logger = logging.getLogger(__name__)


async def create_producer() -> AIOKafkaProducer:
    producer = AIOKafkaProducer(
        bootstrap_servers=settings.kafka_brokers,
        value_serializer=lambda v: json.dumps(v).encode("utf-8"),
    )
    await producer.start()
    logger.info("kafka 生产者已启动")
    return producer


async def send_group_reply(
    producer: AIOKafkaProducer,
    user_id: str,
    content: str,
    message_id: str,
    conversation_id: str = "",
) -> None:
    """以 AI 用户身份发送群消息回复。"""
    now_ms = int(datetime.now(timezone.utc).timestamp() * 1000)

    data = GroupChatData(
        group_id=settings.ai_user_id,
        sender_id=settings.ai_user_id,
        content=content,
        timestamp=now_ms,
        message_id=message_id,
        conversation_id=conversation_id,
    )

    event = BusinessEvent(
        common_params=CommonParams(
            event_type="user.chat.group",
            event_name="user.chat.group",
            event_id=message_id,
            timestamp=datetime.now(timezone.utc).isoformat(),
        ),
        data=data.model_dump(),
    )

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
    conversation_id: str = "",
) -> None:
    await send_group_reply(
        producer, user_id, "[AI 暂时无法回复，请稍后重试]",
        f"err-{message_id}", conversation_id,
    )
