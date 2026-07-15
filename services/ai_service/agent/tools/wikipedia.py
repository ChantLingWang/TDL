"""Wikipedia 知识库工具 —— MediaWiki API，支持代理。"""

import logging
import urllib.parse

import httpx

logger = logging.getLogger(__name__)

_ENDPOINT = "https://en.wikipedia.org/w/api.php"
_HEADERS = {"User-Agent": "ChantAI/1.0 (research-agent; https://chant.app)"}
_CLIENT: httpx.AsyncClient | None = None


def _get_client() -> httpx.AsyncClient:
    """延迟创建 httpx.AsyncClient，注入代理（如有）。"""
    global _CLIENT
    if _CLIENT is not None:
        return _CLIENT
    from config.settings import settings
    proxy = settings.http_proxy or None
    _CLIENT = httpx.AsyncClient(
        timeout=httpx.Timeout(15.0),
        headers=_HEADERS,
        proxy=proxy,
    )
    if proxy:
        logger.info("Wikipedia 走代理 %s", proxy)
    return _CLIENT


async def _close_client() -> None:
    global _CLIENT
    if _CLIENT is not None:
        await _CLIENT.aclose()
        _CLIENT = None


async def search_wikipedia(query: str, max_results: int = 5) -> list[dict]:
    """搜索 Wikipedia，返回标题 + URL + 完整 intro extract。

    两步：先搜索找到页面标题，再逐页拉取正文摘要。
    """
    client = _get_client()
    params = {
        "action": "query",
        "format": "json",
        "list": "search",
        "srsearch": query,
        "srlimit": max_results,
    }
    try:
        resp = await client.get(_ENDPOINT, params=params)
        resp.raise_for_status()
        data = resp.json()
    except Exception as e:
        logger.warning("Wikipedia 搜索失败 query=%s err=%s", query, e)
        return []

    search_results = data.get("query", {}).get("search", [])
    titles = [item["title"] for item in search_results]

    # 批量拉取页面 extract
    extracts = await _batch_extracts(client, titles)

    results: list[dict] = []
    for item in search_results:
        title = item["title"]
        pageid = item["pageid"]
        extract = extracts.get(str(pageid), "") or _strip_html(item.get("snippet", ""))

        results.append({
            "title": title,
            "url": f"https://en.wikipedia.org/wiki/{urllib.parse.quote(title.replace(' ', '_'))}",
            "summary": extract,
            "pageid": pageid,
        })

    return results


async def _batch_extracts(client: httpx.AsyncClient, titles: list[str]) -> dict[str, str]:
    """批量获取多篇文章的 intro extract。"""
    if not titles:
        return {}
    params = {
        "action": "query",
        "format": "json",
        "titles": "|".join(titles),
        "prop": "extracts",
        "exintro": 1,
        "explaintext": 1,
        "exlimit": len(titles),
    }
    try:
        resp = await client.get(_ENDPOINT, params=params)
        resp.raise_for_status()
        data = resp.json()
    except Exception as e:
        logger.debug("Wikipedia batch extract 失败: %s", e)
        return {}

    pages = data.get("query", {}).get("pages", {})
    return {pid: info.get("extract", "") for pid, info in pages.items()}


def _strip_html(text: str) -> str:
    import re
    return re.sub(r"<[^>]+>", "", text)
