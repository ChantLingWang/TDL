"""Qdrant 客户端初始化和集合管理。"""

import logging

from qdrant_client import QdrantClient
from qdrant_client.models import Distance, VectorParams

from config.settings import settings
from embedding.config import EMBEDDING_DIM

logger = logging.getLogger(__name__)

_client: QdrantClient | None = None


async def init() -> None:
    """初始化 Qdrant 连接并创建 collection（幂等）。"""
    global _client

    url = settings.qdrant_url
    collection = settings.qdrant_collection

    _client = QdrantClient(url=url, timeout=10)

    existing = [c.name for c in _client.get_collections().collections]
    if collection not in existing:
        _client.create_collection(
            collection_name=collection,
            vectors_config=VectorParams(size=EMBEDDING_DIM, distance=Distance.COSINE),
        )
        logger.info("Qdrant collection 已创建: %s (dim=%d)", collection, EMBEDDING_DIM)
    else:
        logger.info("Qdrant 已连接: %s, collection=%s, 已有 %d 个 collection",
                    url, collection, len(existing))


def get_client() -> QdrantClient:
    """获取已初始化的 Qdrant 客户端。"""
    if _client is None:
        raise RuntimeError("Qdrant 未初始化")
    return _client


async def close() -> None:
    """关闭 Qdrant 连接（QdrantClient 自动管理连接池，通常无需显式关闭）。"""
    global _client
    if _client is not None:
        _client.close()
        _client = None
