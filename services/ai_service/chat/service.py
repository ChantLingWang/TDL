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

from chat.memory.sliding_window import SlidingWindowMemory
from config.settings import settings
from shared.cost.store import insert_cost
from shared.cost.tracker import compute_cost
from shared.kafka.producer import send_ai_reply, send_error_reply
from shared.llm.factory import get_llm
from shared.llm.base import LLMMessage
from shared.llm.router import route_chat
from shared.models import ChatHistoryMessage

logger = logging.getLogger(__name__)

from chat.prompts import SYSTEM_PROMPT

_memories: dict[str, SlidingWindowMemory] = {}


def _get_memory(user_id: str) -> SlidingWindowMemory:
    if user_id not in _memories:
        _memories[user_id] = SlidingWindowMemory()
    return _memories[user_id]


@retry(
    retry=retry_if_exception_type(httpx.HTTPError),
    stop=stop_after_attempt(2),
    wait=wait_exponential(multiplier=1, min=1, max=5),
)
async def _fetch_history(
    conversation_id: str, limit: int = 30,
) -> list[ChatHistoryMessage]:
    """调用 chat_service 拉取会话历史。"""
    url = f"{settings.chat_service_url}/api/v1/messages/history"
    params = {
        "conversation_id": conversation_id,
        "limit": limit,
        "cursor": int(time.time()),  # Unix 秒级时间戳
    }
    async with httpx.AsyncClient(timeout=httpx.Timeout(15.0)) as client:
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
    memory = _get_memory(user_id)

    # ---- 加载历史 ----
    try:
        # 私聊会话 ID 是双方 ID 排序后的组合（与 chat_service 的 GenerateSessionID 一致）
        conversation_id = group_id or "_".join(sorted([target, user_id]))
        history = await _fetch_history(conversation_id, limit=30)
        for h in history:
            role = "assistant" if h.sender_id == settings.ai_user_id else "user"
            memory.add(role, h.content)
    except Exception as e:
        logger.warning("拉取历史消息失败 user=%s err=%s", user_id, e)

    # ---- 当前消息加入窗口 ----
    memory.add("user", content)

    # ---- 调用 LLM ----
    llm_messages: list[LLMMessage] = memory.build(SYSTEM_PROMPT)
    try:
        response = await route_chat(llm_messages)
    except Exception:
        logger.exception("LLM 调用失败 user=%s", user_id)
        await send_error_reply(producer, user_id, msg_id, group_id=group_id)
        return

    # ---- 记录成本 ----
    try:
        usage = response.usage
        prompt_tok = usage.get("prompt_tokens", 0)
        completion_tok = usage.get("completion_tokens", 0)
        total_tok = usage.get("total_tokens", prompt_tok + completion_tok)
        model = response.model or settings.llm_provider

        llm = get_llm()
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

    # ---- AI 回复存入记忆 ----
    memory.add("assistant", response.content)

    # ---- 通过 Kafka 发送回复 ----
    await send_ai_reply(
        producer, user_id, response.content, msg_id, group_id=group_id,
        reply_to_msg_id=msg_id,
    )
