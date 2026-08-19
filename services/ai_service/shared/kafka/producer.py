"""Kafka 生产者模块（proto 版）。"""

import logging
from datetime import datetime, timezone

from aiokafka import AIOKafkaProducer

from config.settings import settings
from shared.proto_adapter import (
    envelope_to_json,
    new_ai_reply,
    new_ai_reply_delta,
    new_envelope,
)

logger = logging.getLogger(__name__)


async def create_producer() -> AIOKafkaProducer:
    producer = AIOKafkaProducer(
        bootstrap_servers=settings.kafka_brokers,
        value_serializer=lambda v: v,  # 直接发送 bytes
    )
    await producer.start()
    logger.info("kafka 生产者已启动")
    return producer


async def send_ai_reply(
    producer: AIOKafkaProducer,
    user_id: str,
    content: str,
    message_id: str,
    group_id: str = '',
    metadata: dict | None = None,
    reply_to_msg_id: str | None = None,
) -> None:
    """以 AI 用户身份发送一条回复。"""
    import time
    ts = int(time.time() * 1000)
    reply = new_ai_reply(
        target_user_id=user_id,
        content=content,
        reply_to_msg_id=reply_to_msg_id or message_id,
        message_id=f'ai-{message_id}',
        group_id=group_id,
        timestamp_ms=ts,
        metadata=metadata,
    )
    envelope = new_envelope('chant.chat.v1.AiReplyGenerated', 'ai-service', reply)
    payload = envelope_to_json(envelope)

    await producer.send(
        topic=settings.kafka_topic,
        key=user_id.encode(),
        value=payload,
    )
    logger.info("AI 回复已发送  to=%s msg_id=%s", user_id, message_id)


async def send_ai_reply_delta(
    producer: AIOKafkaProducer,
    user_id: str,
    reply_to_msg_id: str,
    message_id: str,
    seq: int,
    kind: str,
    content: str,
    group_id: str = '',
    metadata: dict | None = None,
) -> None:
    """发送一条 AI 回复的流式增量分块（不落库，仅实时转发）。"""
    import time
    ts = int(time.time() * 1000)
    delta = new_ai_reply_delta(
        target_user_id=user_id,
        reply_to_msg_id=reply_to_msg_id,
        message_id=message_id,
        seq=seq,
        kind=kind,
        content=content,
        group_id=group_id,
        timestamp_ms=ts,
        metadata=metadata,
    )
    envelope = new_envelope('chant.chat.v1.AiReplyDelta', 'ai-service', delta)
    payload = envelope_to_json(envelope)

    await producer.send(
        topic=settings.kafka_topic,
        key=user_id.encode(),  # 与 send_ai_reply 同 key：同分区保序
        value=payload,
    )


async def send_error_reply(
    producer: AIOKafkaProducer,
    user_id: str,
    message_id: str,
    group_id: str = '',
) -> None:
    await send_ai_reply(
        producer, user_id, '[AI 暂时无法回复，请稍后重试]', f'err-{message_id}',
        group_id=group_id, reply_to_msg_id=message_id,
    )
