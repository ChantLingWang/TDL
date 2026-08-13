"""SearXNG 网页搜索 + trafilatura 内容提取工具。

走 HTML 接口（绕过 bot detection 对 JSON 的限制），用 BeautifulSoup 解析结果页。
"""

import asyncio
import ipaddress
import logging
import socket
from urllib.parse import urlparse

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


def _is_safe_url(url: str) -> bool:
    """判断 URL 是否允许抓取：只允许 http/https，且目标地址不能是私网/回环/链路本地。

    搜索结果的 URL 由用户查询词间接控制，不校验会被用作 SSRF 打内网服务。
    """
    try:
        parsed = urlparse(url)
    except ValueError:
        return False

    if parsed.scheme not in ("http", "https"):
        return False

    host = parsed.hostname
    if not host:
        return False

    try:
        ip = ipaddress.ip_address(host)
        ips = {ip}
    except ValueError:
        # 域名：解析所有 A/AAAA 记录后逐个校验，防止 DNS 指向内网
        try:
            infos = socket.getaddrinfo(host, None)
        except socket.gaierror:
            return False
        ips = set()
        for info in infos:
            try:
                ips.add(ipaddress.ip_address(info[4][0]))
            except ValueError:
                continue
        if not ips:
            return False

    for addr in ips:
        if addr.version == 6 and addr.ipv4_mapped is not None:
            addr = addr.ipv4_mapped
        if (
            addr.is_private
            or addr.is_loopback
            or addr.is_link_local
            or addr.is_reserved
            or addr.is_multicast
            or addr.is_unspecified
        ):
            return False
    return True


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
    if not _is_safe_url(url):
        logger.debug("拒绝抓取非安全地址 url=%s", url[:60])
        return ""
    try:
        async with httpx.AsyncClient(proxy=proxy, timeout=httpx.Timeout(10.0),
                                      headers={"User-Agent": "ChantAI/1.0"},
                                      follow_redirects=False) as client:
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
