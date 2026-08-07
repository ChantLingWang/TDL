"""成本记录持久化模块。"""

import logging
from datetime import datetime, timedelta, timezone

import asyncpg
from config.settings import settings

logger = logging.getLogger(__name__)
_pool: asyncpg.Pool | None = None

CST = timezone(timedelta(hours=8))

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


async def _ensure_partitions(conn: asyncpg.Connection) -> None:
    """按 llm_api_costs 的分区表设计，自动补建当月与下月分区（YYYYMM）。

    分区命名约定：{table}_{YYYYMM}
    分区边界：每月 1 日 00:00 +08 到次月 1 日 00:00 +08。
    """
    table = settings.cost_table_name
    now = datetime.now(CST)

    for offset in (0, 1):
        total = now.year * 12 + (now.month - 1) + offset
        year, month_index = divmod(total, 12)
        month = month_index + 1

        next_total = total + 1
        next_year, next_month_index = divmod(next_total, 12)
        next_month = next_month_index + 1

        partition_name = f"{table}_{year:04d}{month:02d}"
        start_bound = f"{year:04d}-{month:02d}-01 00:00:00+08"
        end_bound = f"{next_year:04d}-{next_month:02d}-01 00:00:00+08"

        sql = (
            f"CREATE TABLE IF NOT EXISTS {partition_name} PARTITION OF {table} "
            f"FOR VALUES FROM ('{start_bound}') TO ('{end_bound}');"
        )
        await conn.execute(sql)
        logger.info("成本表分区已就绪: %s", partition_name)


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
        try:
            await _ensure_partitions(conn)
        except Exception:
            logger.exception("自动创建成本表分区失败，成本记录可能无法写入")
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
