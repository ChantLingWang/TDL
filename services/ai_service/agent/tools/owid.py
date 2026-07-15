"""Our World in Data 统计数据查询工具。

旧版 api.ourworldindata.org/v1 已于 2024 年废弃。
现改用 OWID 站内 API：搜索 → 图表元数据。
"""

import asyncio
import logging
import httpx

logger = logging.getLogger(__name__)

_SEARCH_URL = "https://ourworldindata.org/search.json"
_GRAPHER_BASE = "https://ourworldindata.org/grapher"
_USER_AGENT = "ChantAI/1.0 (https://chant.app)"


def _get_proxy() -> str | None:
    from config.settings import settings
    return settings.http_proxy or None


async def search_owid(query: str, max_results: int = 5) -> list[dict]:
    """搜索 OWID 数据指标。

    两步：
      1. 调用搜索 JSON 接口，获取匹配的 chart slug 列表
      2. 并发拉取每个 chart 的元数据，提取标题/来源/变量信息
    """
    proxy = _get_proxy()

    async with httpx.AsyncClient(
        proxy=proxy,
        timeout=httpx.Timeout(15.0),
        headers={"User-Agent": _USER_AGENT},
    ) as client:
        # ---- 搜索 ----
        try:
            resp = await client.get(_SEARCH_URL, params={"q": query})
            resp.raise_for_status()
            data = resp.json()
        except Exception as e:
            logger.warning("OWID 搜索失败 query=%s err=%s", query, e)
            return []

        charts = data.get("charts", [])
        if not charts:
            logger.info("OWID: 无匹配图表 query='%s'", query)
            return []

        # ---- 并发拉取图表元数据 ----
        async def _fetch_meta(slug: str) -> dict | None:
            try:
                r = await client.get(f"{_GRAPHER_BASE}/{slug}.metadata.json")
                r.raise_for_status()
                return r.json()
            except Exception as e:
                logger.debug("OWID meta 失败 slug=%s err=%s", slug, e)
                return None

        slugs = [c["slug"] for c in charts[: max_results * 2]]
        metas = await asyncio.gather(*[_fetch_meta(s) for s in slugs],
                                      return_exceptions=True)

    # ---- 组装结果 ----
    results: list[dict] = []
    for chart, meta in zip(charts, metas):
        if not meta or isinstance(meta, BaseException):
            continue
        slug = chart["slug"]
        title = chart.get("title") or meta.get("title", slug)
        subtitle = chart.get("subtitle") or meta.get("subtitle", "")
        source_name = ""
        if isinstance(meta.get("source"), dict):
            source_name = meta["source"].get("name", "")

        # 变量信息：名称 + 单位 + 描述
        var_parts: list[str] = []
        for dim in meta.get("dimensions", []):
            for var in dim.get("variables", []):
                disp = var.get("display", {})
                name = disp.get("name") or var.get("name", "")
                unit = disp.get("unit") or ""
                desc = var.get("description") or ""
                line = name
                if unit:
                    line += f"({unit})"
                if desc:
                    line += f" - {desc}"
                if line:
                    var_parts.append(line)

        lines = [f"标题: {title}"]
        if subtitle:
            lines.append(f"说明: {subtitle}")
        if source_name:
            lines.append(f"来源: {source_name}")
        if var_parts:
            lines.append(f"变量: {' | '.join(var_parts)}")

        results.append({
            "title": title,
            "url": f"{_GRAPHER_BASE}/{slug}",
            "summary": "\n".join(lines),
        })

        if len(results) >= max_results:
            break

    logger.info("OWID: query='%s' found=%d", query, len(results))
    return results
