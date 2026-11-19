"""Kafka 消费者模块。

拉取与处理解耦：
    consume_loop  只负责解析消息 + 放入有界队列
    worker        从队列取消息处理；同一 conversation_id 串行，不同会话并行
"""

from __future__ import annotations

import asyncio
import logging
from contextlib import asynccontextmanager
from dataclasses import dataclass

from aiokafka import AIOKafkaConsumer

from config.settings import settings
from infra.proto_adapter import parse_envelope

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


@dataclass
class _LockEntry:
    """一把会话锁 + 当前持有/等待它的 worker 数。"""

    lock: asyncio.Lock
    refs: int = 0


class _KeyedLocks:
    """按 key 复用 asyncio.Lock，并在无人使用时回收，避免字典无限增长。"""

    def __init__(self) -> None:
        self._entries: dict[str, _LockEntry] = {}

    @asynccontextmanager
    async def hold(self, key: str):
        entry = self._entries.get(key)
        if entry is None:
            entry = _LockEntry(asyncio.Lock())
            self._entries[key] = entry
        entry.refs += 1
        try:
            async with entry.lock:
                yield
        finally:
            entry.refs -= 1
            if entry.refs == 0:
                self._entries.pop(key, None)


async def consume_loop(consumer: AIOKafkaConsumer, handler) -> None:
    """拉取消息并交给 worker 处理。

    - 拉取协程只做 parse + put，不执行业务；
    - 队列有界，满时暂停拉取（背压）；
    - 同一 conversation_id 串行，不同会话并行；
    - 暂时沿用自动提交（阶段一，只验证并发；手动提交在下一阶段）。
    """
    queue: asyncio.Queue = asyncio.Queue(maxsize=max(1, settings.ai_queue_maxsize))
    locks = _KeyedLocks()
    worker_count = max(1, settings.ai_worker_count)

    workers = [
        asyncio.create_task(
            _worker(queue, handler, locks), name=f"ai-worker-{index}"
        )
        for index in range(worker_count)
    ]
    logger.info(
        "消费 worker 已启动  count=%d queue_max=%d", worker_count, queue.maxsize
    )

    cancelled = False
    try:
        async for msg in consumer:
            try:
                envelope = parse_envelope(msg.value)
            except Exception:
                logger.exception("消息解析失败，已跳过")
                continue
            await queue.put((envelope, _message_key(msg)))
    except asyncio.CancelledError:
        cancelled = True
    finally:
        if not cancelled:
            # consumer 正常结束（例如测试/停机）：等队列排空再收 worker
            await queue.join()
        for worker in workers:
            worker.cancel()
        await asyncio.gather(*workers, return_exceptions=True)
        await consumer.stop()


async def _worker(
    queue: asyncio.Queue,
    handler,
    locks: _KeyedLocks,
) -> None:
    while True:
        envelope, conversation_id = await queue.get()
        try:
            async with locks.hold(conversation_id):
                await handler(envelope)
        except Exception:
            # 单条消息失败不能弄死 worker，后面的消息还要继续处理
            logger.exception("消息处理失败 conversation=%s", conversation_id or "-")
        finally:
            queue.task_done()


def _message_key(msg) -> str:
    """gateway 用 conversation_id 作为 Kafka key；缺失时退化为全局串行。"""
    key = getattr(msg, "key", None)
    if isinstance(key, bytes):
        return key.decode("utf-8", "replace")
    if key is None:
        return ""
    return str(key)


__all__ = ["consume_loop", "create_consumer"]
