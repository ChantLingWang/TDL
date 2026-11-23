"""proto adapter —— 封装 proto 消息的 JSON 编解码。"""

import time
import uuid

from google.protobuf.json_format import Parse, MessageToJson

from chant.common.v1 import envelope_pb2 as _envelope
from chant.chat.v1 import event_pb2 as _event


# ---- 反序列化（消费端）----

def parse_envelope(raw: str | bytes) -> _envelope.EventEnvelope:
    """从 Kafka 消息 JSON 解析事件信封。"""
    return Parse(raw, _envelope.EventEnvelope())


def parse_message_sent(env: _envelope.EventEnvelope) -> _event.MessageSent:
    """从信封中解析 MessageSent。"""
    return Parse(env.data, _event.MessageSent())


# ---- 序列化（生产端）----

def envelope_to_json(env: _envelope.EventEnvelope) -> bytes:
    """将事件信封序列化为 JSON bytes。"""
    return MessageToJson(env, preserving_proto_field_name=True).encode('utf-8')


def new_envelope(event_type: str, source: str, msg) -> _envelope.EventEnvelope:
    """创建一个事件信封。"""
    data_json = MessageToJson(msg, preserving_proto_field_name=True)
    return _envelope.EventEnvelope(
        event_id=f"{event_type}-{uuid.uuid4()}",
        event_type=event_type,
        source=source,
        timestamp=int(time.time() * 1000),
        data=data_json.encode('utf-8'),
    )


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
    payload: str,
) -> _event.AiTurnStep:
    """构造一条 AI step 事件。payload 是 JSON 字符串。"""
    return _event.AiTurnStep(
        sender_id=sender_id,
        target_user_id=target_user_id,
        conversation_id=conversation_id,
        reply_to_msg_id=reply_to_msg_id,
        message_id=message_id,
        round=round_no,
        step=step,
        attempt=attempt,
        kind=kind,
        payload=payload,
    )
