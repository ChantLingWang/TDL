import time
"""Chat 模式服务 —— 用户与 AI 对话的核心处理逻辑。

完整链路：
    1. 从 chat_service 拉取历史消息 → 回填滑动窗口
    2. 将当前用户消息加入窗口
    3. 调用 LLM 生成回复（含重试）
    4. 将 AI 回复存入窗口
    5. 通过 Kafka 发送回复 → chat_service → MongoDB + WS → 用户
"""

import logging
import uuid

import httpx
from tenacity import (
    retry,
    retry_if_exception_type,
    stop_after_attempt,
    wait_exponential,
)

from chat.memory.context import assemble_context
from config.settings import settings
from shared.cost.store import insert_cost
from shared.cost.tracker import compute_cost
from shared.kafka.producer import (
    send_ai_reply,
    send_ai_reply_delta,
    send_error_reply,
)
from shared.llm.factory import get_llm
from shared.models import ChatHistoryMessage

logger = logging.getLogger(__name__)

from chat.prompts import SYSTEM_PROMPT


@retry(
    retry=retry_if_exception_type(httpx.HTTPError),
    stop=stop_after_attempt(2),
    wait=wait_exponential(multiplier=1, min=1, max=5),
)
async def _fetch_history(
    conversation_id: str, limit: int = 50,
) -> list[ChatHistoryMessage]:
    """调用 chat_service 拉取会话历史（返回新 → 旧）。"""
    url = f"{settings.chat_service_url}/api/v1/messages/history"
    params = {
        "conversation_id": conversation_id,
        "limit": limit,
        "cursor": int(time.time()),  # Unix 秒级时间戳
    }
    # trust_env=False：内部调用直连，不经过环境里的 HTTP 代理
    # （代理只用于访问外部 LLM API，本地/内网服务走代理会被错误拦截）
    async with httpx.AsyncClient(timeout=httpx.Timeout(15.0), trust_env=False) as client:
        resp = await client.get(
            url, params=params,
            headers={"X-Internal-Key": settings.internal_api_key},
        )
        resp.raise_for_status()
        body = resp.json()
    messages_raw = body.get("messages") or []
    return [ChatHistoryMessage(**m) for m in messages_raw]


async def handle_private_message(producer, event_data: dict) -> None:
    """收到发给 ai-assistant 的消息后，执行完整的处理流程。"""
    user_id = event_data.get("sender_id", "")
    target = event_data.get("target_user_id", "")
    content = event_data.get("content", "")
    msg_id = event_data.get("message_id", str(uuid.uuid4()))
    group_id = event_data.get("group_id", "")

    # 安全断言：确保只处理目标为 AI 的消息
    if not group_id and target != settings.ai_user_id:
        return
    logger.info("收到 AI 消息  from=%s group=%s msg=%s", user_id, group_id, msg_id)

    # ---- 加载历史（接口返回新 → 旧，摆正为旧 → 新）----
    history: list[ChatHistoryMessage] = []
    try:
        # 私聊会话 ID 是双方 ID 排序后的组合（与 chat_service 的 GenerateSessionID 一致）
        conversation_id = group_id or "_".join(sorted([target, user_id]))
        history = await _fetch_history(conversation_id, limit=50)
        history.reverse()
    except Exception as e:
        logger.warning("拉取历史消息失败 user=%s err=%s", user_id, e)

    # ---- 组装上下文（DSH 式：折叠摘要 + 检查点后尾巴 + 当前消息）----
    llm_messages = await assemble_context(
        conversation_id, SYSTEM_PROMPT, history, content,
        current_msg_id=msg_id,
    )

    # ---- 流式调用 LLM，增量分块经 Kafka 推送 ----
    # 回复 ID 提前确定：所有 delta 与最终 AiReplyGenerated 共用，
    # 前端按 (message_id, seq) 把分块累积到同一条回复上。
    reply_id = f'ai-{msg_id}'
    reasoning_parts: list[str] = []
    content_parts: list[str] = []
    usage: dict = {}
    seq = 0

    async def _flush(kind: str, buf: list[str]) -> None:
        """把同 kind 的缓冲拼成一块发出去。"""
        nonlocal seq
        if not buf:
            return
        await send_ai_reply_delta(
            producer, user_id, msg_id, reply_id, seq, kind, ''.join(buf),
            group_id=group_id,
        )
        seq += 1
        buf.clear()

    try:
        llm = get_llm()
        # 起步进度：让前端思考块立刻出现（无思考内容时也有反馈）
        await _flush('progress', ['正在思考…'])
        buf_kind = ''
        buf: list[str] = []
        async for chunk in llm.chat_stream(llm_messages):
            if chunk.usage:
                usage = chunk.usage
                continue
            if not chunk.text:
                continue
            if chunk.kind != buf_kind:
                await _flush(buf_kind, buf)
                buf_kind = chunk.kind
            buf.append(chunk.text)
            (reasoning_parts if chunk.kind == 'reasoning' else content_parts).append(chunk.text)
            # 约 16 字符合一块：既保证流式体验，又不打爆 Kafka
            if len(buf) >= 16:
                await _flush(buf_kind, buf)
        await _flush(buf_kind, buf)
        await send_ai_reply_delta(
            producer, user_id, msg_id, reply_id, seq, 'done', '',
            group_id=group_id,
        )
    except Exception:
        logger.exception("LLM 流式调用失败 user=%s", user_id)
        await send_error_reply(producer, user_id, msg_id, group_id=group_id)
        return

    content = ''.join(content_parts)
    reasoning = ''.join(reasoning_parts)
    if not content:
        logger.error("流式回复为空 user=%s", user_id)
        await send_error_reply(producer, user_id, msg_id, group_id=group_id)
        return

    # ---- 记录成本 ----
    try:
        prompt_tok = usage.get("prompt_tokens", 0)
        completion_tok = usage.get("completion_tokens", 0)
        total_tok = usage.get("total_tokens", prompt_tok + completion_tok)
        model = getattr(llm, 'model_name', settings.llm_provider)

        pricing = llm.get_pricing(model)
        input_price, output_price, cost_usd = compute_cost(
            pricing, prompt_tok, completion_tok,
        )
        await insert_cost(
            user_id=user_id, provider=settings.llm_provider, model=model,
            prompt_tokens=prompt_tok, completion_tokens=completion_tok,
            total_tokens=total_tok, input_price=input_price,
            output_price=output_price, cost_usd=cost_usd, message_id=msg_id,
        )
    except Exception as e:
        logger.warning("成本记录失败 user=%s msg=%s err=%s", user_id, msg_id, e)

    # ---- 通过 Kafka 发送最终回复（整块落库；思考全文随 metadata 持久化）----
    metadata = {}
    if reasoning:
        metadata['reasoning'] = reasoning
    await send_ai_reply(
        producer, user_id, content, msg_id, group_id=group_id,
        reply_to_msg_id=msg_id, metadata=metadata,
    )
