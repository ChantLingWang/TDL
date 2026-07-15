"""SearXNG 网页搜索 + trafilatura 内容提取工具。

走 HTML 接口（绕过 bot detection 对 JSON 的限制），用 BeautifulSoup 解析结果页。
"""

import asyncio
import logging

import httpx
from bs4 import BeautifulSoup
import trafilatura

logger = logging.getLogger(__name__)

# trafilatura 正文提取失败时打 ERROR 是噪音，降级处理
logging.getLogger("trafilatura").setLevel(logging.WARNING)

_SEARXNG_URL: str | None = None


def _get_url() -> str:
    global _SEARXNG_URL
    if _SEARXNG_URL is None:
        from config.settings import settings
        _SEARXNG_URL = settings.searxng_base_url.rstrip("/")
    return _SEARXNG_URL


def _get_proxy() -> str | None:
    from config.settings import settings
    return settings.http_proxy or None


async def search_searxng(query: str, max_results: int = 5) -> list[dict]:
    """搜索 SearXNG（HTML 模式）并抓取网页正文。"""
    proxy = _get_proxy()
    params = {"q": query}

    async with httpx.AsyncClient(proxy=proxy, timeout=httpx.Timeout(15.0),
                                  headers={"User-Agent": "ChantAI/1.0"}) as client:
        try:
            resp = await client.get(f"{_get_url()}/search", params=params)
            resp.raise_for_status()
        except Exception as e:
            logger.warning("SearXNG 搜索失败 query=%s err=%s", query, e)
            return []

        # 从 HTML 中解析搜索结果
        raw = _parse_results(resp.text, max_results)

    # 并行抓取正文
    tasks = [_fetch_content(proxy, r["url"]) for r in raw]
    contents = await asyncio.gather(*tasks, return_exceptions=True)

    results: list[dict] = []
    for i, (r, content) in enumerate(zip(raw, contents)):
        results.append({
            "title": r["title"],
            "url": r["url"],
            "snippet": r.get("snippet", "")[:300],
            "content": content if isinstance(content, str) else "",
        })
    return results


def _parse_results(html: str, max_results: int) -> list[dict]:
    """从 SearXNG HTML 结果页解析搜索条目。"""
    soup = BeautifulSoup(html, "lxml")
    articles = soup.find_all("article", class_="result")
    results: list[dict] = []
    for a in articles[:max_results]:
        title_el = a.find("h3") or a.find("a")
        title = title_el.get_text(strip=True) if title_el else ""
        link_el = a.find("a", href=True)
        url = link_el["href"] if link_el else ""
        snippet_el = a.find("p", class_="content") or a.find("p")
        snippet = snippet_el.get_text(strip=True) if snippet_el else ""
        if title and url:
            results.append({"title": title, "url": url, "snippet": snippet})
    return results


async def _fetch_content(proxy: str | None, url: str) -> str:
    """trafilatura 提取网页正文前 800 字。"""
    if not url:
        return ""
    try:
        async with httpx.AsyncClient(proxy=proxy, timeout=httpx.Timeout(10.0),
                                      headers={"User-Agent": "ChantAI/1.0"}) as client:
            resp = await client.get(url)
            resp.raise_for_status()
            html = resp.text
    except Exception as e:
        logger.debug("抓取失败 url=%s err=%s", url[:60], e)
        return ""
    try:
        extracted = trafilatura.extract(html, include_comments=False,
                                        include_tables=True, output_format="txt")
        return extracted[:800] if extracted else ""
    except Exception:
        return ""
