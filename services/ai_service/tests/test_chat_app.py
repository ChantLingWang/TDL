"""apps.chat.service 装配测试：假 producer、假 LLM，不触网。"""

import asyncio
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from apps.chat import service  # noqa: E402
from llm_relay.types import LLMResult, Message, Usage  # noqa: E402


class FakeLLM:
    """返回一个完整的最终回答，并记录每次请求的 tools。"""

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


def test_handle_message_runs_agent_loop_and_sends_final(monkeypatch):
    sent: list[tuple] = []
    fake_llm = FakeLLM()

    async def fake_assemble(conversation_id, system_prompt, history, content, current_msg_id=""):
        assert conversation_id == "conv-1"
        assert content == "hi"
        assert history == []
        assert "工具目录" in system_prompt
        return [Message("user", "hi")]

    async def fake_delta(
        producer, user_id, msg_id, reply_id, seq, kind, content,
        conversation_id="",
    ):
        sent.append(("delta", kind, content, seq, conversation_id))

    async def fake_reply(producer, user_id, content, message_id, **kwargs):
        sent.append(("final", content, kwargs.get("metadata", {}), kwargs.get("conversation_id", "")))

    async def fake_insert(**kwargs):
        sent.append(("cost", kwargs["total_tokens"]))

    monkeypatch.setattr(service, "assemble_context", fake_assemble)
    monkeypatch.setattr(service, "llm", fake_llm)
    monkeypatch.setattr(service, "send_ai_reply_delta", fake_delta)
    monkeypatch.setattr(service, "send_ai_reply", fake_reply)
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
            },
        )
    )

    kinds = [item[1] for item in sent if item[0] == "delta"]
    assert "progress" in kinds
    assert "thinking" in kinds
    assert "content" in kinds
    assert "done" in kinds
    assert "".join(item[1] for item in sent if item[0] == "final") == "你好"
    assert any(item[0] == "cost" and item[1] == 12 for item in sent)

    # 所有 delta 都携带同一个 conversation_id
    assert {item[4] for item in sent if item[0] == "delta"} == {"conv-1"}

    # AgentLoop 第一次请求带的是常驻工具 load_tools
    tools = fake_llm.calls[0]["tools"]
    assert [item.name for item in tools] == ["load_tools"]

    metadata = next(item[2] for item in sent if item[0] == "final")
    assert metadata["reasoning"] == "想一下"

    final_conversation = next(item[3] for item in sent if item[0] == "final")
    assert final_conversation == "conv-1"
