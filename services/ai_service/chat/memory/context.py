"""DSH 式短期记忆 —— 真相在历史，折叠用检查点，组装按 token 预算。

对齐 DSH 的四条原则：
    1. 单一真相源：对话内容只存在于 chat_service 的历史（Mongo），
       本模块不持有任何对话消息；
    2. 投影组装：模型看到的上下文 = 系统提示 + 折叠摘要 + 检查点之后的尾巴；
    3. 折叠压缩：超出预算时，旧段送 LLM 生成摘要，摘要幂等写入 Mongo 检查点
       （带 watermark，重启可重放恢复，与 DSH 的 checkpoint 折叠同构）；
    4. 无进程内状态：重启 / 多实例都从检查点恢复记忆。

token 估算（无 tokenizer 依赖的近似）：CJK 字符 1 字 1 token，其余 4 字符 1 token。
"""

import logging
import time
from datetime import datetime

from config.settings import settings
from shared.llm.base import LLMMessage
from shared.llm.factory import get_llm
from shared.models import ChatHistoryMessage

logger = logging.getLogger(__name__)

# motor 惰性初始化（与 cost_store 的 asyncpg 模式一致，连接失败不阻塞聊天）
_motor_client = None
_motor_db = None


def _get_db():
    global _motor_client, _motor_db
    if _motor_client is None:
        from motor.motor_asyncio import AsyncIOMotorClient
        _motor_client = AsyncIOMotorClient(
            settings.memory_mongo_url, serverSelectionTimeoutMS=3000)
        _motor_db = _motor_client[settings.memory_mongo_db]
    return _motor_db


# ── token 估算 ──────────────────────────────────────────────────────────────


def _is_cjk(ch: str) -> bool:
    cp = ord(ch)
    return (0x4E00 <= cp <= 0x9FFF      # CJK 统一表意
            or 0x3400 <= cp <= 0x4DBF   # 扩展 A
            or 0xF900 <= cp <= 0xFAFF   # 兼容表意
            or 0x3040 <= cp <= 0x30FF)  # 假名


def estimate_tokens(text: str) -> int:
    """粗略估算 token 数。"""
    cjk = sum(1 for ch in text if _is_cjk(ch))
    return cjk + (len(text) - cjk + 3) // 4


def _to_epoch_ms(ts: int | str | None) -> int:
    """历史消息 timestamp（毫秒数字或 ISO 字符串）归一化为 epoch 毫秒。"""
    if ts is None:
        return 0
    if isinstance(ts, int):
        return ts
    try:
        dt = datetime.fromisoformat(str(ts).replace("Z", "+00:00"))
        return int(dt.timestamp() * 1000)
    except Exception:
        return 0


# ── 折叠检查点（Mongo，幂等 upsert）────────────────────────────────────────


async def load_checkpoint(conversation_id: str) -> dict | None:
    """读取会话的折叠检查点；Mongo 不可用时返回 None（记忆降级但不阻塞）。"""
    try:
        db = _get_db()
        return await db[settings.memory_checkpoint_collection].find_one(
            {"conversation_id": conversation_id})
    except Exception as e:
        logger.warning("检查点读取失败 conv=%s err=%s", conversation_id, e)
        return None


async def save_checkpoint(conversation_id: str, watermark_ms: int, summary: str) -> None:
    """幂等写入折叠检查点。"""
    try:
        db = _get_db()
        await db[settings.memory_checkpoint_collection].update_one(
            {"conversation_id": conversation_id},
            {"$set": {
                "watermark_ms": watermark_ms,
                "summary": summary,
                "updated_at_ms": int(time.time() * 1000),
            }},
            upsert=True,
        )
    except Exception as e:
        logger.warning("检查点写入失败 conv=%s err=%s", conversation_id, e)


# ── 摘要 ────────────────────────────────────────────────────────────────────


SUMMARY_PROMPT = (
    "你是对话记忆压缩助手。把下面的对话历史压缩成一段简洁的中文摘要，"
    "必须保留：1) 用户的关键信息与偏好；2) 已经讨论并达成结论的事实；"
    "3) 尚未完成的任务或遗留问题。尽量精炼，但不要遗漏重要事实。"
    "\n\n对话历史：\n{history}"
)


async def _summarize(history_text: str) -> str:
    """把旧段历史压缩成摘要（独立 LLM 调用，对应 DSH 的 summarization model）。"""
    llm = get_llm()
    response = await llm.chat(
        [LLMMessage(role="user", content=SUMMARY_PROMPT.format(history=history_text[-32000:]))],
        max_tokens=settings.memory_summary_max_tokens, temperature=0.3,
    )
    return response.content.strip()


# ── 组装 ────────────────────────────────────────────────────────────────────


def format_history(msgs: list[ChatHistoryMessage]) -> str:
    """把历史消息格式化为摘要用的文本。"""
    lines: list[str] = []
    for h in msgs:
        who = "用户" if h.sender_id != settings.ai_user_id else "AI"
        lines.append(f"{who}: {h.content}")
    return "\n".join(lines)


async def assemble_context(
    conversation_id: str,
    system_prompt: str,
    history: list[ChatHistoryMessage],
    current_text: str,
    current_msg_id: str = "",
) -> list[LLMMessage]:
    """组装 LLM 上下文（旧 → 新），当前消息在末尾。

    触发条件（对齐 DSH）：summary + 尾巴 超过预算的 80% 时折叠，
    旧段摘要化，只保留最近约 16% 预算的原文尾巴。
    """
    # 对齐 DSH：阈值与保留量都从模型上下文窗口按比例推导
    budget = settings.memory_context_window
    trigger = int(budget * settings.memory_summary_trigger_ratio)
    retain = int(budget * settings.memory_retain_ratio)

    cp = await load_checkpoint(conversation_id)
    watermark = cp.get("watermark_ms", 0) if cp else 0
    summary = cp.get("summary", "") if cp else ""

    # 检查点之后的尾巴（旧 → 新）；剔除当前消息自身（它已被 chat_service 落库）
    tail = [
        h for h in history
        if _to_epoch_ms(h.timestamp) > watermark and h.message_id != current_msg_id
    ]

    # ── 触发折叠 ──
    tail_tokens = sum(estimate_tokens(h.content) for h in tail)
    if tail and estimate_tokens(summary) + tail_tokens > trigger:
        # 从最新往回保留 retain 预算，其余旧段送摘要
        keep_from = len(tail)
        kept = 0
        for i in range(len(tail) - 1, -1, -1):
            t = estimate_tokens(tail[i].content)
            if kept + t > retain and i < len(tail) - 1:
                break
            kept += t
            keep_from = i
        old_part = tail[:keep_from]
        if old_part:
            try:
                history_text = format_history(old_part)
                merged = (summary + "\n\n" + history_text) if summary else history_text
                new_summary = await _summarize(merged)
                if new_summary:
                    watermark_ms = max(_to_epoch_ms(h.timestamp) for h in old_part)
                    await save_checkpoint(conversation_id, watermark_ms, new_summary)
                    summary, tail = new_summary, tail[keep_from:]
            except Exception as e:
                # 摘要失败不阻塞对话，本次按原文继续
                logger.warning("摘要生成失败，跳过本次折叠 conv=%s err=%s", conversation_id, e)

    # ── 拼装 ──
    messages: list[LLMMessage] = []
    if system_prompt:
        messages.append(LLMMessage(role="system", content=system_prompt))
    if summary:
        messages.append(LLMMessage(role="user", content=f"[更早对话摘要]\n{summary}"))
    for h in tail:
        role = "assistant" if h.sender_id == settings.ai_user_id else "user"
        messages.append(LLMMessage(role=role, content=h.content))
    messages.append(LLMMessage(role="user", content=current_text))
    return messages
