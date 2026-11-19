"""infra.kafka.consumer 并发消费测试：假 consumer + 假 handler，不触网。"""

import asyncio
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from infra.kafka import consumer as consumer_mod  # noqa: E402


class FakeMessage:
    def __init__(self, envelope, key):
        self.value = envelope
        self.key = key


class FakeConsumer:
    def __init__(self, messages):
        self._messages = list(messages)
        self.stopped = False

    def __aiter__(self):
        return self._iterate()

    async def _iterate(self):
        for message in self._messages:
            yield message

    async def stop(self):
        self.stopped = True


def _run(messages, handler):
    async def main():
        consumer = FakeConsumer(messages)
        await consumer_mod.consume_loop(consumer, handler)
        return consumer

    return asyncio.run(main())


def test_different_conversations_run_concurrently(monkeypatch):
    monkeypatch.setattr(consumer_mod, "parse_envelope", lambda value: value)

    active = 0
    peak = 0

    async def handler(envelope):
        nonlocal active, peak
        active += 1
        peak = max(peak, active)
        await asyncio.sleep(0.05)
        active -= 1

    messages = [FakeMessage(index, f"conv-{index}".encode()) for index in range(4)]
    consumer = _run(messages, handler)

    assert peak >= 2
    assert consumer.stopped is True


def test_same_conversation_is_serial(monkeypatch):
    monkeypatch.setattr(consumer_mod, "parse_envelope", lambda value: value)

    order = []

    async def handler(envelope):
        order.append(f"start-{envelope}")
        await asyncio.sleep(0.02)
        order.append(f"end-{envelope}")

    messages = [FakeMessage(index, b"conv-1") for index in range(2)]
    _run(messages, handler)

    assert order == ["start-0", "end-0", "start-1", "end-1"]


def test_handler_error_does_not_stop_worker(monkeypatch):
    monkeypatch.setattr(consumer_mod, "parse_envelope", lambda value: value)

    processed = []

    async def handler(envelope):
        if envelope == 0:
            raise RuntimeError("boom")
        processed.append(envelope)

    messages = [FakeMessage(0, b"conv-1"), FakeMessage(1, b"conv-1")]
    _run(messages, handler)

    assert processed == [1]


def test_parse_error_is_skipped(monkeypatch):
    def parse(value):
        if value == "bad":
            raise ValueError("bad envelope")
        return value

    monkeypatch.setattr(consumer_mod, "parse_envelope", parse)

    processed = []

    async def handler(envelope):
        processed.append(envelope)

    messages = [FakeMessage("bad", b"conv-1"), FakeMessage("good", b"conv-1")]
    _run(messages, handler)

    assert processed == ["good"]
