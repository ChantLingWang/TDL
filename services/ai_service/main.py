"""ai_service —— 后台 Kafka 消费者。

职责：
    消费 chat_service 发来的私聊消息 → 路由到 chat / agent 模块 →
    调用 LLM 生成回复 → 通过 Kafka 发回 chat_service → 前端收到。

架构要点：
    - 无 HTTP 端口，纯后台 worker
    - 使用独立的 Kafka consumer group（ai_service_group），和 chat_service 互不争抢
    - 优雅关闭：SIGINT / SIGTERM → 停止消费 → 关闭 producer → 退出
"""

import asyncio
import logging
import signal

from aiokafka import AIOKafkaConsumer, AIOKafkaProducer

from chat.service import handle_private_message
from shared.cost import store as cost_store
from config.settings import settings
from shared.kafka.consumer import consume_loop, create_consumer
from shared.kafka.producer import create_producer
from shared.models import BusinessEvent

logger = logging.getLogger(__name__)

AI_USER_ID = settings.ai_user_id


async def dispatch(producer: AIOKafkaProducer, event: BusinessEvent) -> None:
    """事件分发入口。

    目前只处理 event_type == "user.chat.private" 且 target 是 AI 用户的消息。
    后续可按消息前缀（如 "/agent"）或 content 特征分流到 agent 模块。
    """
    etype = event.common_params.event_type
    # Kafka BusinessEvent 的 data 字段可能是 dict 或已解析对象
    data = event.data if isinstance(event.data, dict) else {}

    # 非私聊消息不处理
    if etype != "user.chat.private":
        return

    # 不是发给 AI 的，忽略（如用户之间的私聊）
    target = data.get("target_user_id", "")
    if target != AI_USER_ID:
        return

    await handle_private_message(producer, data)


async def main() -> None:
    """服务入口：初始化 → 创建 Kafka 消费/生产连接 → 进入消费循环 → 等待关闭信号。"""
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s  %(levelname)-7s %(name)s  %(message)s",
    )
    logger.info("ai_service 启动中  ai_user_id=%s", AI_USER_ID)

    consumer: AIOKafkaConsumer | None = None
    producer: AIOKafkaProducer | None = None

    # 用 asyncio.Event 阻塞主协程，收到信号后 set 触发退出
    stop_event = asyncio.Event()

    def _signal_handler() -> None:
        logger.info("收到关闭信号")
        stop_event.set()

    loop = asyncio.get_running_loop()
    loop.add_signal_handler(signal.SIGINT, _signal_handler)
    loop.add_signal_handler(signal.SIGTERM, _signal_handler)

    try:
        # 先建立连接；任一失败会抛异常退出
        consumer = await create_consumer()
        producer = await create_producer()

        # 初始化成本审计数据库连接池
        await cost_store.init_pool()

        async def handler(event: BusinessEvent) -> None:
            await dispatch(producer, event)

        # consumer 在后台运行，主协程等待关闭信号
        consumer_task = asyncio.create_task(consume_loop(consumer, handler))

        await stop_event.wait()
        consumer_task.cancel()
        try:
            await consumer_task
        except asyncio.CancelledError:
            pass
    finally:
        # 确保资源清理
        if consumer:
            await consumer.stop()
        if producer:
            await producer.stop()
        await cost_store.close_pool()
        logger.info("ai_service 已停止")


if __name__ == "__main__":
    asyncio.run(main())
