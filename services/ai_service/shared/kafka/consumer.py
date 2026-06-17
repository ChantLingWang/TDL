"""Kafka 消费者模块。

设计：
    - 使用独立的 consumer group，不会和 chat_service 争抢消息
    - consume_loop 是永久阻塞的异步循环，由上层 cancel 来停止
    - 每条消息反序列化为 BusinessEvent 后交给 handler 回调
"""

import asyncio
import json
import logging

from aiokafka import AIOKafkaConsumer

from config.settings import settings
from shared.models import BusinessEvent

logger = logging.getLogger(__name__)


async def create_consumer() -> AIOKafkaConsumer:
    """创建并启动 aiokafka 消费者。

    auto_offset_reset="latest"：只消费新消息，不回溯历史。
    enable_auto_commit=True：自动提交 offset，简化错误处理。
    """
    consumer = AIOKafkaConsumer(
        settings.kafka_topic,
        bootstrap_servers=settings.kafka_brokers,
        group_id=settings.kafka_group_id,
        value_deserializer=lambda m: json.loads(m.decode("utf-8")),
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


async def consume_loop(
    consumer: AIOKafkaConsumer,
    handler,
) -> None:
    """死循环消费 Kafka 消息，每条消息反序列化后调用 handler。

    handler 签名为 async callable(BusinessEvent)。
    单条消息处理异常不会中断循环，仅打日志。
    上层通过 cancel 协程 + CancelledError 优雅退出。
    """
    try:
        async for msg in consumer:
            try:
                raw = msg.value
                # 反序列化为 Pydantic 模型，校验格式
                event = BusinessEvent(**raw)
                await handler(event)
            except Exception:
                logger.exception("消息处理异常，已跳过")
    except asyncio.CancelledError:
        # 正常退出信号，不需要打日志
        pass
    finally:
        await consumer.stop()
