"""成本记录持久化模块。"""

import logging
import asyncpg
from config.settings import settings

logger = logging.getLogger(__name__)
_pool: asyncpg.Pool | None = None

CREATE_TABLE_SQL = """
CREATE TABLE IF NOT EXISTS {table} (
    id                BIGSERIAL,
    user_id           TEXT NOT NULL,
    provider          TEXT NOT NULL,
    model             TEXT NOT NULL,
    prompt_tokens     INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    total_tokens      INTEGER NOT NULL,
    input_price       DOUBLE PRECISION NOT NULL,
    output_price      DOUBLE PRECISION NOT NULL,
    cost_usd          DOUBLE PRECISION NOT NULL,
    message_id        TEXT,
    created_at        TIMESTAMPTZ DEFAULT NOW()
);
"""

INSERT_SQL = """
INSERT INTO {table}
    (user_id, provider, model,
     prompt_tokens, completion_tokens, total_tokens,
     input_price, output_price, cost_usd, message_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);
"""


async def init_pool() -> asyncpg.Pool:
    global _pool
    _pool = await asyncpg.create_pool(
        host=settings.cost_db_host,
        port=settings.cost_db_port,
        user=settings.cost_db_user,
        password=settings.cost_db_password,
        database=settings.cost_db_name,
        min_size=1,
        max_size=5,
    )
    async with _pool.acquire() as conn:
        await conn.execute(CREATE_TABLE_SQL.format(table=settings.cost_table_name))
    logger.info("cost db pool ready  table=%s", settings.cost_table_name)
    return _pool


async def close_pool() -> None:
    global _pool
    if _pool:
        await _pool.close()
        _pool = None


async def insert_cost(
    *,
    user_id: str,
    provider: str,
    model: str,
    prompt_tokens: int,
    completion_tokens: int,
    total_tokens: int,
    input_price: float,
    output_price: float,
    cost_usd: float,
    message_id: str = "",
) -> None:
    if not settings.cost_tracking_enabled:
        return
    async with _pool.acquire() as conn:
        await conn.execute(
            INSERT_SQL.format(table=settings.cost_table_name),
            user_id, provider, model,
            prompt_tokens, completion_tokens, total_tokens,
            input_price, output_price, cost_usd, message_id,
        )
