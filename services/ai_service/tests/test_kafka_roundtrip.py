"""Kafka 连通性测试 —— chat_service 格式 → 生产 → 消费验证。

用法:
    cd services/ai_service
    pip install aiokafka pydantic
    python tests/test_kafka_roundtrip.py

前提:
    Kafka 在 localhost:9094 运行，topic chat_group_message 已存在。
    chat_service 和 ai_service 无需启动。
"""

import asyncio
import json
import sys
import os
from datetime import datetime, timezone

# 将 ai_service 根目录加入 sys.path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from aiokafka import AIOKafkaConsumer, AIOKafkaProducer

KAFKA_BROKERS = "localhost:9094"
KAFKA_TOPIC = "chat_group_message"
TEST_GROUP = "ai_service_test_group"

# ---- 模拟 chat_service 发送的消息格式（和 Go 侧完全一致） ----

TEST_EVENT = {
    "common_params": {
        "event_type": "user.chat.private",
        "event_name": "user.chat.private",
        "event_id": "test-roundtrip-001",
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "execution_mode": "",
    },
    "data": {
        "sender_id": "test-user",
        "target_user_id": "ai-assistant",
        "content": "你好，这是跨服务连通性测试",
        "timestamp": int(datetime.now(timezone.utc).timestamp() * 1000),
        "message_id": "test-roundtrip-001",
        "message_type": "text",
    },
}


async def test_produce() -> dict:
    """生产一条模拟 chat_service 私聊消息到 Kafka。"""
    producer = AIOKafkaProducer(
        bootstrap_servers=KAFKA_BROKERS,
        value_serializer=lambda v: json.dumps(v).encode("utf-8"),
    )
    await producer.start()
    await producer.send(
        topic=KAFKA_TOPIC,
        key=b"ai-assistant",
        value=TEST_EVENT,
    )
    await producer.stop()
    print("✅ produce 成功: target=ai-assistant content=你好...")
    return TEST_EVENT


async def test_consume(expected_event: dict) -> dict | None:
    """消费并验证消息格式。"""
    consumer = AIOKafkaConsumer(
        KAFKA_TOPIC,
        bootstrap_servers=KAFKA_BROKERS,
        group_id=TEST_GROUP,
        value_deserializer=lambda m: json.loads(m.decode("utf-8")),
        auto_offset_reset="earliest",  # 从最早开始读，拿到刚生产的消息
        enable_auto_commit=False,
    )
    await consumer.start()

    try:
        # 轮询最多 10 秒
        async for msg in consumer:
            data = msg.value
            event_type = data.get("common_params", {}).get("event_type", "")
            if event_type != "user.chat.private":
                continue

            inner = data.get("data", {})
            if inner.get("target_user_id") != "ai-assistant":
                continue

            print(f"✅ consume 成功: type={event_type} sender={inner.get('sender_id')}")

            # ---- 格式校验 ----
            errors = []
            if "common_params" not in data:
                errors.append("缺少 common_params")
            else:
                cp = data["common_params"]
                for field in ("event_type", "event_name", "event_id", "timestamp"):
                    if field not in cp:
                        errors.append(f"common_params 缺少 {field}")

            if "data" not in data:
                errors.append("缺少 data")
            else:
                d = data["data"]
                for field in ("sender_id", "target_user_id", "content", "message_id"):
                    if field not in d:
                        errors.append(f"data 缺少 {field}")

            if errors:
                print(f"❌ 格式校验失败: {errors}")
            else:
                print("✅ 消息格式与 Go 侧 BusinessEvent 一致")

            return data
    except asyncio.TimeoutError:
        print("❌ consume 超时（10 秒内未收到消息）")
        return None
    finally:
        await consumer.stop()


async def main():
    print("=" * 50)
    print("  Kafka 跨服务连通性测试")
    print(f"  brokers={KAFKA_BROKERS}  topic={KAFKA_TOPIC}")
    print("=" * 50)

    produced = await test_produce()
    await asyncio.sleep(1)  # 等消息落盘
    consumed = await test_consume(produced)

    if consumed:
        print("\n" + "=" * 50)
        print("  ✅ 测试通过: chat_service 格式 → Kafka 通道正常")
        print("  ai_service 消费此消息后将调用 DeepSeek 生成回复")
        print("=" * 50)
    else:
        print("\n❌ 测试失败")
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(main())
