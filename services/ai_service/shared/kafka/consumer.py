"""Kafka 消费者模块（proto 版）。"""

import asyncio
import logging

from aiokafka import AIOKafkaConsumer

from config.settings import settings
from shared.proto_adapter import parse_envelope

logger = logging.getLogger(__name__)


async def create_consumer() -> AIOKafkaConsumer:
    consumer = AIOKafkaConsumer(
        settings.kafka_topic,
        bootstrap_servers=settings.kafka_brokers,
        group_id=settings.kafka_group_id,
        value_deserializer=lambda m: m.decode("utf-8"),  # 保持原始字符串
        auto_offset_reset="latest",
        enable_auto_commit=True,
    )
    await consumer.start()
    logger.info(
        "kafka 消费者已启动  topic=%s group=%s brokers=%s",
        settings.kafka_topic,
        settings.kafka_group_id,
        settings.kafka_brokers,
    )
    return consumer


async def consume_loop(consumer: AIOKafkaConsumer, handler) -> None:
    """死循环消费 Kafka 消息，解析为 proto EventEnvelope 后调用 handler。"""
    try:
        async for msg in consumer:
            try:
                envelope = parse_envelope(msg.value)
                await handler(envelope)
            except Exception:
                logger.exception("消息处理异常，已跳过")
    except asyncio.CancelledError:
        pass
    finally:
        await consumer.stop()
