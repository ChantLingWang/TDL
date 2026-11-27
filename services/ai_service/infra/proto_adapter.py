"""proto adapter —— 事件信封与 step 的二进制编解码。

Kafka value = EventEnvelope 的 protobuf 二进制；
EventEnvelope.data = 具体事件的 protobuf 二进制。
只有一层编码，没有 JSON 字符串/base64。
"""

import json
import time
import uuid

from google.protobuf import json_format, struct_pb2

from chant.common.v1 import envelope_pb2 as _envelope
from chant.chat.v1 import event_pb2 as _event


# ---- 反序列化（消费端）----

def parse_envelope(raw: bytes) -> _envelope.EventEnvelope:
    """从 Kafka 二进制解析事件信封。"""
    return _envelope.EventEnvelope.FromString(raw)


def parse_message_sent(env: _envelope.EventEnvelope) -> _event.MessageSent:
    """从信封中解析 MessageSent。"""
    return _event.MessageSent.FromString(env.data)


# ---- 序列化（生产端）----

def envelope_to_bytes(env: _envelope.EventEnvelope) -> bytes:
    """将事件信封序列化为 protobuf 二进制。"""
    return env.SerializeToString()


def new_envelope(event_type: str, source: str, msg) -> _envelope.EventEnvelope:
    """创建一个事件信封；data 是 msg 的 protobuf 二进制。"""
    return _envelope.EventEnvelope(
        event_id=f"{event_type}-{uuid.uuid4()}",
        event_type=event_type,
        source=source,
        timestamp=int(time.time() * 1000),
        data=msg.SerializeToString(),
    )


# ---- step 构造 ----

def _to_value(data) -> struct_pb2.Value:
    """把任意 JSON 数据转成 google.protobuf.Value。"""
    value = struct_pb2.Value()
    json_format.Parse(json.dumps(data, ensure_ascii=False, default=str), value)
    return value


_TOOL_STATUS = {
    "completed": _event.TOOL_STATUS_COMPLETED,
    "failed": _event.TOOL_STATUS_FAILED,
    "timeout": _event.TOOL_STATUS_TIMEOUT,
}

_RESULT_STATUS = {
    "completed": _event.RESULT_STATUS_COMPLETED,
    "canceled": _event.RESULT_STATUS_CANCELED,
    "failed": _event.RESULT_STATUS_FAILED,
}


def new_turn_step(
    sender_id: str,
    target_user_id: str,
    conversation_id: str,
    reply_to_msg_id: str,
    message_id: str,
    round_no: int,
    step: int,
    attempt: int,
    kind: str,
    payload: dict,
) -> _event.AiTurnStep:
    """按 kind 把 payload 填进 typed oneof。"""
    turn_step = _event.AiTurnStep(
        sender_id=sender_id,
        target_user_id=target_user_id,
        conversation_id=conversation_id,
        reply_to_msg_id=reply_to_msg_id,
        message_id=message_id,
        round=round_no,
        step=step,
        attempt=attempt,
    )

    if kind == "THINK":
        turn_step.think.think = str(payload.get("think") or "")
        return turn_step

    if kind == "TOOL_CALL":
        for item in payload.get("calls") or []:
            call = turn_step.tool_call.calls.add()
            call.id = str(item.get("id") or "")
            call.name = str(item.get("name") or "")
            call.input.CopyFrom(_to_value(item.get("input")))
        return turn_step

    if kind == "TOOL_RESULT":
        for item in payload.get("results") or []:
            result = turn_step.tool_result.results.add()
            result.id = str(item.get("id") or "")
            result.name = str(item.get("name") or "")
            result.payload.CopyFrom(_to_value(item.get("payload")))
            result.status = _TOOL_STATUS.get(
                str(item.get("status") or ""), _event.TOOL_STATUS_UNSPECIFIED
            )
            result.duration_ms = int(item.get("duration_ms") or 0)
            result.error = str(item.get("error") or "")
        return turn_step

    if kind == "RESULT":
        turn_step.result.result = str(payload.get("result") or "")
        turn_step.result.status = _RESULT_STATUS.get(
            str(payload.get("status") or ""), _event.RESULT_STATUS_UNSPECIFIED
        )
        turn_step.result.reason = str(payload.get("reason") or "")
        return turn_step

    raise ValueError(f"未知 step kind: {kind}")
