"""端到端测试 —— 启动 ai_service，发消息，验证 AI 回复。

用法:
    cd services/ai_service
    python tests/test_e2e_roundtrip.py

前提:
    Kafka localhost:9094 运行中
    不需要 chat_service
"""

import asyncio
import json
import os
import signal
import subprocess
import sys
import time
from datetime import datetime, timezone

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from aiokafka import AIOKafkaConsumer, AIOKafkaProducer

KAFKA_BROKERS = "localhost:9094"
KAFKA_TOPIC = "chat_group_message"
AI_USER = "ai-assistant"

TIMESTAMP = int(datetime.now(timezone.utc).timestamp() * 1000)
MSG_ID = f"e2e-{TIMESTAMP}"


async def consume_ai_reply(timeout: int = 30) -> dict | None:
    """消费 Kafka，寻找 AI 发出的回复消息。"""
    consumer = AIOKafkaConsumer(
        KAFKA_TOPIC,
        bootstrap_servers=KAFKA_BROKERS,
        group_id=f"e2e_test_group_{TIMESTAMP}",
        value_deserializer=lambda m: json.loads(m.decode("utf-8")),
        auto_offset_reset="latest",
        enable_auto_commit=True,
    )
    await consumer.start()

    deadline = asyncio.get_event_loop().time() + timeout
    try:
        async for msg in consumer:
            data = msg.value
            inner = data.get("data", {})

            # 只找 AI 发出的回复
            if inner.get("sender_id") == AI_USER:
                return inner

            # 超时检查
            if asyncio.get_event_loop().time() > deadline:
                return None
    finally:
        await consumer.stop()
    return None


async def send_test_message() -> None:
    """模拟 chat_service 发送一条发给 AI 的私聊消息。"""
    producer = AIOKafkaProducer(
        bootstrap_servers=KAFKA_BROKERS,
        value_serializer=lambda v: json.dumps(v).encode("utf-8"),
    )
    await producer.start()

    event = {
        "common_params": {
            "event_type": "user.chat.private",
            "event_name": "user.chat.private",
            "event_id": MSG_ID,
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "execution_mode": "",
        },
        "data": {
            "sender_id": "e2e-test-user",
            "target_user_id": AI_USER,
            "content": "hello，请用中文回复：1+1等于几？",
            "timestamp": TIMESTAMP,
            "message_id": MSG_ID,
            "message_type": "text",
        },
    }

    await producer.send(topic=KAFKA_TOPIC, key=b"ai-assistant", value=event)
    await producer.stop()
    print(f"✅ 已发送测试消息  msg_id={MSG_ID}")


async def main():
    print("=" * 55)
    print("  AI Service 端到端往返测试")
    print("  Kafka → ai_service → DeepSeek → Kafka")
    print("=" * 55)

    # 1. 启动 ai_service 子进程
    ai_dir = os.path.join(os.path.dirname(__file__), "..")
    proc = subprocess.Popen(
        [sys.executable, "main.py"],
        cwd=ai_dir,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    print(f"✅ ai_service 已启动 pid={proc.pid}，等待初始化（5秒）...")
    await asyncio.sleep(5)

    try:
        # 2. 先启动监听消费者（必须在发送消息之前）
        print("⏳ 启动监听消费者...")
        consume_task = asyncio.create_task(consume_ai_reply(timeout=30))

        # 3. 发送测试消息（consumer 已就绪）
        await send_test_message()

        # 4. 等待 AI 回复
        print("⏳ 等待 AI 回复（最多 30 秒）...")
        reply = await consume_task

        if reply is None:
            print("\n❌ 超时：未收到 AI 回复")
            print("\n--- ai_service 日志 ---")
            proc.terminate()
            await asyncio.sleep(1)
            log = proc.stdout.read() if proc.stdout else ""
            print(log[:2000])
            sys.exit(1)

        print(f"\n✅ 收到 AI 回复:")
        print(f"   sender: {reply.get('sender_id')}")
        print(f"   target: {reply.get('target_user_id')}")
        print(f"   content: {reply.get('content', '')[:100]}...")

        # 格式校验
        errors = []
        if reply.get("sender_id") != AI_USER:
            errors.append(f"sender 应为 {AI_USER}，实际 {reply.get('sender_id')}")
        if reply.get("target_user_id") != "e2e-test-user":
            errors.append("target 应为 e2e-test-user")
        if not reply.get("content"):
            errors.append("content 为空")

        if errors:
            print(f"\n❌ 格式校验失败: {errors}")
            sys.exit(1)
        else:
            print("✅ 消息格式校验通过")

        print("\n" + "=" * 55)
        print("  ✅ 端到端测试通过")
        print("  chat_service 格式 → Kafka → ai_service → Kafka → 格式正确")
        print("=" * 55)

    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()


if __name__ == "__main__":
    asyncio.run(main())
