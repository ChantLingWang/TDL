"""Chat 模式服务 —— 用户与 AI 对话的核心处理逻辑。

完整链路：
    1. 从 chat_service 拉取历史消息 → 回填滑动窗口
    2. 将当前用户消息加入窗口
    3. 调用 LLM 生成回复（含重试）
    4. 将 AI 回复存入窗口
    5. 通过 Kafka 发送回复 → chat_service → WS → 用户
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

# AI 助理的系统提示词：定义 AI 的基本人设和行为风格
SYSTEM_PROMPT = (
    "你是一个乐于助人的 AI 助手，名叫 Chant AI。请简洁自然地回答问题。"
)


# ---------------------------------------------------------------------------
# 每个用户独立的滑动窗口记忆（进程内存，重启后丢失为正常行为）
# ---------------------------------------------------------------------------
_memories: dict[str, SlidingWindowMemory] = {}


def _get_memory(user_id: str) -> SlidingWindowMemory:
    """获取或创建用户的滑动窗口记忆实例。"""
    if user_id not in _memories:
        _memories[user_id] = SlidingWindowMemory()
    return _memories[user_id]


# ---------------------------------------------------------------------------
# 从 chat_service HTTP API 拉取历史消息
# ---------------------------------------------------------------------------
@retry(
    retry=retry_if_exception_type(httpx.HTTPError),
    stop=stop_after_attempt(2),
    wait=wait_exponential(multiplier=1, min=1, max=5),
)
async def _fetch_history(
    user_id: str, limit: int = 30
) -> list[ChatHistoryMessage]:
    """调用 chat_service 的 GET /api/v1/messages/history 获取对话历史。

    使用 chat_service 而非直连 MongoDB 的原因：
        - 干净的服务边界：数据主权归 chat_service
        - 统一鉴权和格式：chat_service 已处理会话 ID 和消息排序
    """
    session_id = _session_id(user_id)
    url = f"{settings.chat_service_url}/api/v1/messages/history"
    params = {"conversation_id": session_id, "limit": limit}
    async with httpx.AsyncClient(timeout=httpx.Timeout(15.0)) as client:
        resp = await client.get(url, params=params)
        resp.raise_for_status()
        body = resp.json()
    messages_raw = body.get("messages", [])
    return [ChatHistoryMessage(**m) for m in messages_raw]


def _session_id(user_id: str) -> str:
    """生成私聊会话 ID，与 chat_service 的 GenerateSessionID 逻辑一致。

    两个用户 ID 排序后用 _ 拼接，确保同一对用户始终对应同一个 session。
    """
    a, b = sorted([user_id, settings.ai_user_id])
    return f"{a}_{b}"


# ---------------------------------------------------------------------------
# 核心处理函数：一条私聊消息的完整生命周期
# ---------------------------------------------------------------------------
async def handle_private_message(producer, event_data: dict) -> None:
    """收到发给 ai-assistant 的私聊消息后，执行完整的处理流程。"""
    user_id = event_data.get("sender_id", "")
    target = event_data.get("target_user_id", "")
    content = event_data.get("content", "")
    msg_id = event_data.get("message_id", str(uuid.uuid4()))

    # 安全断言：确保只处理目标为 AI 的消息
    if target != settings.ai_user_id:
        return

    logger.info("收到 AI 消息  from=%s msg=%s", user_id, msg_id)
    memory = _get_memory(user_id)

    # ---- 第 1 步：加载历史消息，回填滑动窗口 ----
    # 这一步可能会因 chat_service 不可用而失败；失败时用已有的窗口记忆继续
    try:
        history = await _fetch_history(user_id, limit=30)
        for h in history:
            role = (
                "assistant" if h.sender_id == settings.ai_user_id else "user"
            )
            memory.add(role, h.content)
    except Exception:
        logger.warning(
            "拉取历史消息失败 user=%s，仅使用内存中的记忆继续", user_id
        )

    # ---- 第 2 步：当前消息加入窗口 ----
    memory.add("user", content)

    # ---- 第 3 步：调用 LLM（内含 3 次重试） ----
    llm_messages: list[LLMMessage] = memory.build(SYSTEM_PROMPT)
    try:
        response = await route_chat(llm_messages)
    except Exception:
        logger.exception("LLM 调用失败 user=%s", user_id)
        # 通知用户 AI 暂时不可用
        await send_error_reply(producer, user_id, msg_id)
        return

    # ---- 第 3.5 步：记录 API 调用成本 ----
    try:
        usage = response.usage
        prompt_tok = usage.get("prompt_tokens", 0)
        completion_tok = usage.get("completion_tokens", 0)
        total_tok = usage.get("total_tokens", prompt_tok + completion_tok)
        model = response.model or settings.llm_model

        llm = get_llm()
        pricing = llm.get_pricing(model)
        input_price, output_price, cost_usd = compute_cost(
            pricing, prompt_tok, completion_tok
        )
        await insert_cost(
            user_id=user_id, provider=settings.llm_provider, model=model,
            prompt_tokens=prompt_tok, completion_tokens=completion_tok,
            total_tokens=total_tok, input_price=input_price,
            output_price=output_price, cost_usd=cost_usd, message_id=msg_id,
        )
    except Exception:
        logger.warning("成本记录失败 user=%s msg=%s", user_id, msg_id)

    # ---- 第 4 步：AI 回复存入记忆，供后续对话使用 ----
    memory.add("assistant", response.content)

    # ---- 第 5 步：通过 Kafka 发送回复 ----
    # chat_service 消费后会自动存入 MongoDB 并推送给前端
    ai_msg_id = f"ai-{msg_id}"
    await send_ai_reply(producer, user_id, response.content, ai_msg_id)
