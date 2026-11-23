"""proto_adapter 二进制信封与 typed payload 往返测试。"""

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from chant.chat.v1 import event_pb2 as chat_event  # noqa: E402
from infra.proto_adapter import (  # noqa: E402
    envelope_to_bytes,
    new_envelope,
    new_turn_step,
    parse_envelope,
    parse_message_sent,
)


def _step(**overrides):
    kwargs = {
        "sender_id": "ai-assistant",
        "target_user_id": "u1",
        "conversation_id": "conv-1",
        "reply_to_msg_id": "m1",
        "message_id": "ai-m1",
        "round_no": 2,
        "step": 1,
        "attempt": 0,
        "kind": "THINK",
        "payload": {"think": "想一下"},
    }
    kwargs.update(overrides)
    return new_turn_step(**kwargs)


def test_binary_envelope_roundtrip():
    step = _step(kind="RESULT", payload={"result": "hi", "status": "completed", "reason": ""})
    raw = envelope_to_bytes(new_envelope("chant.chat.v1.AiTurnStep", "ai-service", step))

    # 二进制 protobuf 不应该以 JSON 的 { 开头
    assert not raw.startswith(b"{")

    env = parse_envelope(raw)
    assert env.event_type == "chant.chat.v1.AiTurnStep"
    out = chat_event.AiTurnStep.FromString(env.data)
    assert out.WhichOneof("payload") == "result"
    assert out.result.result == "hi"
    assert out.result.status == chat_event.RESULT_STATUS_COMPLETED
    assert out.round == 2
    assert out.step == 1


def test_message_sent_binary_roundtrip():
    sent = chat_event.MessageSent(
        sender_id="u1",
        target_user_id="ai-assistant",
        conversation_id="conv-1",
        content="hi",
        message_id="m1",
        round=5,
    )
    env = new_envelope("chant.chat.v1.MessageSent", "chat-service", sent)
    out = parse_message_sent(parse_envelope(envelope_to_bytes(env)))
    assert out.content == "hi"
    assert out.round == 5


def test_tool_result_value_roundtrip():
    step = _step(
        step=2,
        kind="TOOL_RESULT",
        payload={
            "results": [{
                "id": "call_1",
                "name": "weather",
                "payload": {"temperature": 25, "type": "windy"},
                "status": "completed",
                "duration_ms": 820,
                "error": "",
            }],
        },
    )
    out = chat_event.AiTurnStep.FromString(step.SerializeToString())
    assert out.WhichOneof("payload") == "tool_result"
    result = out.tool_result.results[0]
    assert result.id == "call_1"
    assert result.status == chat_event.TOOL_STATUS_COMPLETED
    assert result.duration_ms == 820
    assert result.payload.struct_value.fields["temperature"].number_value == 25
