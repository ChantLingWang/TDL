"""会话历史查询工具 —— 供 intent 节点查询过往研究报告。

通过 chat_service 的 HTTP API 获取历史消息，筛选 AI 研究型回复。
"""

import logging
import httpx

logger = logging.getLogger(__name__)


async def query_history(
    group_id: str,
    limit: int = 10,
) -> list[dict]:
    """查询指定会话中 AI 研究型回复的历史记录。

    Args:
        group_id: 会话 ID（对应 chat_service 的 conversation_id）
        limit: 最大返回条数

    Returns:
        [{question, answer_summary, domain, timestamp}, ...]
    """
    from config.settings import settings

    url = f"{settings.chat_service_url}/api/v1/messages/history"
    import time as _time
    params = {
        "conversation_id": group_id,
        "limit": limit * 2,  # 多拉一些，过滤后可能不够
        "cursor": int(_time.time()),
    }

    async with httpx.AsyncClient(timeout=httpx.Timeout(10.0)) as client:
        try:
            resp = await client.get(url, params=params)
            resp.raise_for_status()
            body = resp.json()
        except Exception as e:
            logger.warning("query_history 请求失败 group=%s err=%s", group_id, e)
            return []

    messages = body.get("messages") or []

    # 筛选 AI 研究型回复并配对 question-answer
    history: list[dict] = []
    pending_question: str | None = None

    for m in messages:
        sender = m.get("sender_id", "")
        content = m.get("content", "")
        timestamp = m.get("timestamp", 0)

        if sender == settings.ai_research_user_id:
            # AI 回复：与最近一条用户问题配对
            summary = content[:300].replace("\n", " ")
            entry = {
                "question": pending_question or "",
                "answer_summary": summary,
                "timestamp": timestamp,
            }
            # metadata 字段（chat_service 更新后可用）
            meta = m.get("metadata") or {}
            if meta:
                entry["domain"] = meta.get("domain", "")
            history.append(entry)
            pending_question = None
        elif sender not in (settings.ai_user_id, settings.ai_research_user_id):
            # 用户消息：记录为 pending question
            pending_question = content[:200]
            if len(history) >= limit:
                break

    logger.info("query_history: group=%s found=%d", group_id, len(history))
    return history


def format_history(history: list[dict]) -> str:
    """将历史记录格式化为 LLM 可读文本。"""
    if not history:
        return "（无历史研究报告）"

    lines = ["此前分析过的内容："]
    for i, h in enumerate(history, 1):
        domain_tag = f"[{h.get('domain', '未知领域')}] " if h.get("domain") else ""
        lines.append(
            f"{i}. {domain_tag}问题：{h['question']}\n"
            f"   摘要：{h['answer_summary']}"
        )
    return "\n".join(lines)
