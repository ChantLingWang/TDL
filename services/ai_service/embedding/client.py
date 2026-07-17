"""SiliconFlow embedding 和 rerank API 调用。"""

import logging

import httpx

from embedding.config import (
    BASE_URL,
    EMBEDDING_API_KEY,
    EMBEDDING_MODEL,
    RERANKER_API_KEY,
    RERANKER_MODEL,
)

logger = logging.getLogger(__name__)


_RETRY_STATUSES = {429, 500, 502, 503}


async def embed(texts: list[str]) -> list[list[float]]:
    """调用 SiliconFlow embeddings API，返回向量列表。"""
    url = f"{BASE_URL}/embeddings"

    async with httpx.AsyncClient(timeout=httpx.Timeout(30.0)) as client:
        for attempt in range(3):
            try:
                resp = await client.post(
                    url,
                    headers={"Authorization": f"Bearer {EMBEDDING_API_KEY}"},
                    json={"model": EMBEDDING_MODEL, "input": texts},
                )
                if resp.status_code in _RETRY_STATUSES and attempt < 2:
                    await _backoff(attempt)
                    continue
                resp.raise_for_status()
                data = resp.json()
                return [d["embedding"] for d in data["data"]]
            except Exception as e:
                if attempt < 2:
                    logger.debug("embedding 重试 %d: %s", attempt + 1, e)
                    await _backoff(attempt)
                    continue
                raise

    return []   # unreachable


async def rerank(query: str, documents: list[str], top_n: int = 5) -> list[dict]:
    """调用 SiliconFlow rerank API，返回 {"index": int, "relevance_score": float} 列表。"""
    if not documents:
        return []

    url = f"{BASE_URL}/rerank"

    async with httpx.AsyncClient(timeout=httpx.Timeout(15.0)) as client:
        for attempt in range(3):
            try:
                resp = await client.post(
                    url,
                    headers={"Authorization": f"Bearer {RERANKER_API_KEY}"},
                    json={
                        "model": RERANKER_MODEL,
                        "query": query,
                        "documents": documents,
                        "top_n": top_n,
                    },
                )
                if resp.status_code in _RETRY_STATUSES and attempt < 2:
                    await _backoff(attempt)
                    continue
                resp.raise_for_status()
                return resp.json()["results"]
            except Exception as e:
                if attempt < 2:
                    logger.debug("rerank 重试 %d: %s", attempt + 1, e)
                    await _backoff(attempt)
                    continue
                raise

    return []


async def _backoff(attempt: int) -> None:
    import asyncio
    await asyncio.sleep(0.5 * (2 ** attempt))
