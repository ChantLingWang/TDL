"""apps.chat.service 装配测试：假 producer、假 LLM，不触网。"""

import asyncio
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from apps.chat import service  # noqa: E402
from chant.chat.v1 import event_pb2 as chat_event  # noqa: E402
from llm_relay.types import LLMResult, Message, Usage  # noqa: E402


class FakeLLM:
    """返回一个完整回答，并记录每次请求的 tools。"""

    def __init__(self):
        self.calls: list[dict] = []

    async def complete(self, messages, **kwargs):
        self.calls.append(dict(kwargs))
        return LLMResult(
            content="你好",
            reasoning="想一下",
            usage=Usage(10, 2, 12),
            cost_usd=0.00001,
            model="deepseek-chat",
            finish_reason="stop",
        )


class BoomLLM:
    async def complete(self, messages, **kwargs):
        raise RuntimeError("boom")


def _run(monkeypatch, llm):
    steps = []
    costs = []

    async def fake_assemble(conversation_id, system_prompt, history, content, current_msg_id=""):
        assert conversation_id == "conv-1"
        assert content == "hi"
        assert history == []
        assert "工具目录" in system_prompt
        return [Message("user", "hi")]

    async def fake_send_step(producer, step):
        steps.append(step)

    async def fake_insert(**kwargs):
        costs.append(kwargs)

    monkeypatch.setattr(service, "assemble_context", fake_assemble)
    monkeypatch.setattr(service, "llm", llm)
    monkeypatch.setattr(service, "send_turn_step", fake_send_step)
    monkeypatch.setattr(service, "insert_cost", fake_insert)

    asyncio.run(
        service.handle_message(
            None,
            {
                "sender_id": "u1",
                "target_user_id": "ai-assistant",
                "conversation_id": "conv-1",
                "content": "hi",
                "message_id": "m1",
                "round": 3,
            },
        )
    )
    return steps, costs


def test_handle_message_emits_step_stream(monkeypatch):
    fake_llm = FakeLLM()
    steps, costs = _run(monkeypatch, fake_llm)

    assert [step.WhichOneof("payload") for step in steps] == ["think", "result"]
    assert [step.step for step in steps] == [1, 2]
    assert steps[0].round == 3
    assert steps[0].conversation_id == "conv-1"
    assert steps[0].reply_to_msg_id == "m1"
    assert steps[0].message_id == "ai-m1"
    assert steps[0].think.think == "想一下"

    assert steps[1].result.result == "你好"
    assert steps[1].result.status == chat_event.RESULT_STATUS_COMPLETED
    assert steps[1].result.reason == ""

    assert costs and costs[0]["total_tokens"] == 12
    assert [item.name for item in fake_llm.calls[0]["tools"]] == ["load_tools"]


def test_handle_message_reports_failure_as_result_step(monkeypatch):
    steps, _ = _run(monkeypatch, BoomLLM())

    assert len(steps) == 1
    assert steps[0].WhichOneof("payload") == "result"
    assert steps[0].step == 1
    assert steps[0].result.status == chat_event.RESULT_STATUS_FAILED
    assert "boom" in steps[0].result.reason
