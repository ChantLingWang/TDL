"""ai_service —— 后台 Kafka 消费者。

只做三件事：
    1. 消费 MessageSent，判断是不是发给 AI 的
    2. 交给 apps/chat 处理
    3. 管理 consumer / producer / 连接池的生命周期
"""

import asyncio
import logging
import signal
import sys
from pathlib import Path

# 将 proto 生成的 Python 代码加入搜索路径
sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "proto" / "gen" / "python"))

from aiokafka import AIOKafkaConsumer, AIOKafkaProducer

from apps.chat.service import handle_message
from config.settings import settings
from infra.cost import store as cost_store
from infra.kafka.consumer import consume_loop, create_consumer
from infra.kafka.producer import create_producer
from infra.proto_adapter import parse_message_sent
import llm_relay.llm_tools  # noqa: F401  启动时装配工具注册表
from llm_relay.client import aclose_providers

logger = logging.getLogger(__name__)

AI_USER_ID = settings.ai_user_id


async def dispatch(producer: AIOKafkaProducer, envelope) -> None:
    """事件分发入口：只处理 MessageSent。"""
    event_type = envelope.event_type

    if event_type == "chant.chat.v1.MessageSent":
        msg = parse_message_sent(envelope)
        if msg.target_user_id != AI_USER_ID:
            return

        data = {
            "sender_id": msg.sender_id,
            "target_user_id": msg.target_user_id or AI_USER_ID,
            "conversation_id": msg.conversation_id,
            "content": msg.content,
            "message_id": msg.message_id,
        }
        await handle_message(producer, data)

    elif event_type == "chant.chat.v1.AiReplyGenerated":
        # AI 自己的回复，忽略避免循环
        pass

    else:
        logger.debug("忽略事件类型: %s", event_type)


async def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s  %(levelname)-7s %(name)s  %(message)s",
    )
    logger.info("ai_service 启动中  ai_user_id=%s", AI_USER_ID)

    consumer: AIOKafkaConsumer | None = None
    producer: AIOKafkaProducer | None = None

    stop_event = asyncio.Event()

    def _signal_handler() -> None:
        logger.info("收到关闭信号")
        stop_event.set()

    loop = asyncio.get_running_loop()
    loop.add_signal_handler(signal.SIGINT, _signal_handler)
    loop.add_signal_handler(signal.SIGTERM, _signal_handler)

    try:
        consumer = await create_consumer()
        producer = await create_producer()

        try:
            await cost_store.init_pool()
        except Exception:
            logger.warning("成本审计数据库连接失败，成本记录功能停用")

        async def handler(envelope) -> None:
            await dispatch(producer, envelope)

        consumer_task = asyncio.create_task(consume_loop(consumer, handler))
        await stop_event.wait()
        consumer_task.cancel()
        try:
            await consumer_task
        except asyncio.CancelledError:
            pass
    finally:
        if consumer:
            await consumer.stop()
        if producer:
            await producer.stop()
        try:
            await cost_store.close_pool()
        except Exception:
            pass
        try:
            await aclose_providers()
        except Exception:
            pass
        logger.info("ai_service 已停止")


if __name__ == "__main__":
    asyncio.run(main())
