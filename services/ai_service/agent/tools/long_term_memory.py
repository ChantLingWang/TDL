"""长期记忆 —— 写入和混合检索（语义 × 时间衰减 × 可选重排）。

写入：
    report 生成后调用 store_memory()，同步写入 Qdrant。
    先调 embedding API 拿到 4096 维向量，再 upsert 到 Qdrant，
    payload 包含 user_id / group_id / question / report_summary / domain 等。

检索：
    intent / analyze 节点调用 retrieve_memories()：
      1. embedding 召回 top-K × 4 候选
      2. 时间衰减 × 语义分 混合排序
      3. 可选 Reranker 精排
    返回按 score 降序的 [{question, report_summary, domain, ...}]。
"""

import asyncio
import logging
import time as _time
from datetime import datetime, timezone, timedelta

from qdrant_client.models import PointStruct, Filter, FieldCondition, MatchValue

from embedding.client import embed, rerank
from qdrant.client import get_client
from config.settings import settings

logger = logging.getLogger(__name__)

CST = timezone(timedelta(hours=8))

# ---- 混合排序参数 ----
SEMANTIC_WEIGHT = 0.6    # 语义相似度权重
TIME_DECAY_RATE = 0.005  # 时间衰减系数（每小时）

# ---- 检索参数 ----
RECALL_MULTIPLIER = 4    # 召回倍数（相对于最终 limit）
RERANK_CANDIDATE_RATIO = 3  # 送入 reranker 的候选数 / 最终 limit


# ==================== 写入 ====================

async def store_memory(
    user_id: str,
    group_id: str,
    question: str,
    report: str,
    domain: str = "",
    methodology: str = "",
) -> None:
    """将一份研究报告写入长期记忆。"""
    report = report.strip()
    if not report:
        return

    # 构造 embedding 文本：Q + A 拼接
    embedding_text = f"Q: {question}\nA: {report[:800]}"

    try:
        vectors = await embed([embedding_text])
    except Exception:
        logger.exception("embedding API 失败，记忆写入跳过 user=%s", user_id)
        return

    vector = vectors[0]
    now = datetime.now(CST).isoformat()

    point = PointStruct(
        id=_generate_id(),
        vector=vector,
        payload={
            "user_id": user_id,
            "group_id": group_id,
            "question": question,
            "report_summary": _truncate(report, 500),
            "report_full": report,
            "domain": domain,
            "methodology": methodology,
            "created_at": now,
        },
    )

    try:
        await asyncio.to_thread(
            get_client().upsert,
            collection_name=settings.qdrant_collection,
            points=[point],
        )
        logger.info("记忆已写入 user=%s question=%s", user_id, question[:50])
    except Exception:
        logger.exception("Qdrant 写入失败 user=%s", user_id)


# ==================== 检索 ====================

async def retrieve_memories(
    user_id: str,
    query: str,
    limit: int = 5,
) -> list[dict]:
    """混合检索长期记忆。

    返回 [{question, report_summary, report_full, domain, methodology,
             created_at, score}, ...]，按 score 降序。
    """
    try:
        query_vectors = await embed([query])
    except Exception:
        logger.exception("embedding API 失败，检索返回空")
        return []

    query_vector = query_vectors[0]
    recall_limit = limit * RECALL_MULTIPLIER

    try:
        response = await asyncio.to_thread(
            get_client().query_points,
            collection_name=settings.qdrant_collection,
            query=query_vector,
            query_filter=Filter(
                must=[FieldCondition(key="user_id", match=MatchValue(value=user_id))]
            ),
            limit=recall_limit,
            with_payload=True,
        )
        results = response.points
    except Exception:
        logger.exception("Qdrant 检索失败 user=%s", user_id)
        return []

    if not results:
        return []

    # ---- 混合排序（语义分 × 时间衰减） ----
    now_ts = _time.time()
    scored: list[dict] = []
    for r in results:
        semantic = r.score                            # Qdrant cosine similarity
        created_at = r.payload.get("created_at", "")
        age_hours = _calc_age_hours(created_at, now_ts)
        time_score = _time_decay(age_hours)
        hybrid = semantic * SEMANTIC_WEIGHT + time_score * (1 - SEMANTIC_WEIGHT)
        scored.append({
            "question": r.payload.get("question", ""),
            "report_summary": r.payload.get("report_summary", ""),
            "report_full": r.payload.get("report_full", ""),
            "domain": r.payload.get("domain", ""),
            "methodology": r.payload.get("methodology", ""),
            "created_at": created_at,
            "score": round(hybrid, 4),
            "semantic": round(semantic, 4),
            "time_score": round(time_score, 4),
        })

    scored.sort(key=lambda x: x["score"], reverse=True)

    # ---- 可选：Reranker 精排 top candidates ----
    if len(scored) > limit:
        rerank_n = min(limit * RERANK_CANDIDATE_RATIO, len(scored))
        top_for_rerank = scored[:rerank_n]
        documents = [m["report_summary"] for m in top_for_rerank]

        try:
            rr = await rerank(query, documents, top_n=limit)
            reranked: list[dict] = []
            for item in rr:
                idx = item["index"]
                if idx < len(top_for_rerank):
                    m = top_for_rerank[idx]
                    m["score"] = round(item["relevance_score"], 4)
                    m.pop("semantic", None)
                    m.pop("time_score", None)
                    reranked.append(m)
            logger.info(
                "检索 user=%s recall=%d rerank_in=%d rerank_out=%d",
                user_id, len(results), len(top_for_rerank), len(reranked),
            )
            return reranked
        except Exception:
            logger.debug("rerank 失败，回退到混合排序")

    logger.info("检索 user=%s recall=%d final=%d (no rerank)", user_id, len(results), min(limit, len(scored)))
    return scored[:limit]


# ==================== helpers ====================

def _generate_id() -> str:
    import uuid
    return str(uuid.uuid4())


def _truncate(text: str, max_len: int) -> str:
    """截断到 max_len，尽量在句末断。"""
    if len(text) <= max_len:
        return text
    cut = text[:max_len]
    # 尝试回溯到最后一个完整句子结束处
    for sep in ("。\n", "。", ".\n", ".", "\n", " "):
        idx = cut.rfind(sep)
        if idx > max_len * 0.6:
            return cut[:idx + len(sep.rstrip("\n"))]
    return cut


def _calc_age_hours(created_at: str, now_ts: float) -> float:
    """计算条目距今的小时数。"""
    try:
        dt = datetime.fromisoformat(created_at)
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=CST)
        return (now_ts - dt.timestamp()) / 3600.0
    except Exception:
        return 9999.0


def _time_decay(hours: float) -> float:
    """指数时间衰减：EXP(-rate * hours)。
    
    24h → 0.887，168h（一周）→ 0.432，720h（一月）→ 0.027。
    """
    import math
    return math.exp(-TIME_DECAY_RATE * hours)


# ==================== 格式化（供 prompt 注入） ====================

def format_memories(memories: list[dict]) -> str:
    """将检索结果格式化为 LLM 可读文本。"""
    if not memories:
        return "（无历史分析记录）"

    lines = ["历史相关分析（按相关度排列）："]
    for i, m in enumerate(memories, 1):
        domain_tag = f"[{m['domain']}] " if m.get("domain") else ""
        method_tag = f" | {m['methodology']}" if m.get("methodology") else ""
        time_tag = ""
        if m.get("created_at"):
            try:
                dt = datetime.fromisoformat(m["created_at"])
                time_tag = f" — {_relative_time(dt)}"
            except Exception:
                pass
        lines.append(
            f"{i}. {domain_tag}{method_tag}{time_tag}\n"
            f"   问题：{m['question']}\n"
            f"   摘要：{m['report_summary']}"
        )
    return "\n".join(lines)


def _relative_time(dt: datetime) -> str:
    now = datetime.now(CST)
    diff = now - dt.replace(tzinfo=CST) if dt.tzinfo is None else now - dt
    days = diff.days
    if days == 0:
        return "今天"
    if days == 1:
        return "昨天"
    if days < 7:
        return f"{days}天前"
    if days < 30:
        return f"{days // 7}周前"
    return f"{days // 30}个月前"


# ==================== 预取（并行优化） ====================

_pending_prefetches: dict[str, "asyncio.Task[list[dict]]"] = {}


async def trigger_prefetch(user_id: str, query: str, limit: int = 5) -> str:
    """启动后台预取，返回 key。不阻塞，检索在后台并行进行。"""
    import uuid
    key = str(uuid.uuid4())
    _pending_prefetches[key] = asyncio.create_task(
        retrieve_memories(user_id=user_id, query=query, limit=limit)
    )
    return key


async def consume_prefetch(key: str) -> list[dict]:
    """等待预取完成，返回结果。幂等 —— 同一 key 只消费一次。"""
    task = _pending_prefetches.pop(key, None)
    if task is None:
        return []
    try:
        return await task
    except Exception:
        logger.exception("预取消费失败 key=%s", key[:8])
        return []
