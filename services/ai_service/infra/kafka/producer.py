"""Kafka 生产者模块（proto 版）。"""

import logging

from aiokafka import AIOKafkaProducer

from config.settings import settings
from infra.proto_adapter import envelope_to_json, new_envelope

logger = logging.getLogger(__name__)


async def create_producer() -> AIOKafkaProducer:
    producer = AIOKafkaProducer(
        bootstrap_servers=settings.kafka_brokers,
        value_serializer=lambda v: v,
    )
    await producer.start()
    logger.info("kafka 生产者已启动")
    return producer


async def send_turn_step(producer: AIOKafkaProducer, step) -> None:
    """发送一条 AiTurnStep；key 用 conversation_id 保证同会话有序。"""
    envelope = new_envelope("chant.chat.v1.AiTurnStep", "ai-service", step)
    payload = envelope_to_json(envelope)
    await producer.send(
        topic=settings.kafka_topic,
        key=step.conversation_id.encode(),
        value=payload,
    )
