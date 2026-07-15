"""proto adapter —— 封装 proto 消息的 JSON 编解码，替换手写的 models.py。"""

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


def parse_ai_reply(env: _envelope.EventEnvelope) -> _event.AiReplyGenerated:
    """从信封中解析 AiReplyGenerated。"""
    return Parse(env.data, _event.AiReplyGenerated())


# ---- 序列化（生产端）----

def envelope_to_json(env: _envelope.EventEnvelope) -> bytes:
    """将事件信封序列化为 JSON bytes，可直接发送到 Kafka。"""
    return MessageToJson(env, preserving_proto_field_name=True).encode('utf-8')


def new_envelope(event_type: str, source: str, msg) -> _envelope.EventEnvelope:
    """创建一个事件信封。"""
    data_json = MessageToJson(msg, preserving_proto_field_name=True)
    return _envelope.EventEnvelope(
        event_type=event_type,
        source=source,
        data=data_json.encode('utf-8'),
    )


def new_message_sent(sender_id: str, content: str, message_id: str,
                     target_user_id: str = '', group_id: str = '',
                     conversation_type: str = '') -> _event.MessageSent:
    """构造一条 MessageSent。"""
    return _event.MessageSent(
        sender_id=sender_id,
        content=content,
        message_id=message_id,
        target_user_id=target_user_id,
        group_id=group_id,
        conversation_type=conversation_type,
    )


def new_ai_reply(target_user_id: str, content: str,
                 reply_to_msg_id: str, message_id: str,
                 group_id: str = '',
                 timestamp_ms: int = 0,
                 metadata: dict | None = None) -> _event.AiReplyGenerated:
    """构造一条 AI 回复。"""
    kwargs: dict = {}
    if timestamp_ms:
        kwargs['timestamp_ms'] = timestamp_ms
    if metadata:
        kwargs['metadata'] = metadata
    return _event.AiReplyGenerated(
        sender_id='ai-assistant',
        target_user_id=target_user_id,
        content=content,
        reply_to_msg_id=reply_to_msg_id,
        message_id=message_id,
        group_id=group_id,
        **kwargs,
    )
