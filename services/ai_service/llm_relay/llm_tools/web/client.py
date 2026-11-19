"""DeepSeek 网页搜索 / 抓取中继。

所有需要联网取网页内容的业务都走这里，业务层不直接调供应商 API。

端点（与 DeepSeek API 同一套 Bearer 鉴权）：
    POST {base_url}/tools/search
    POST {base_url}/tools/fetch

文档：https://api-docs.deepseek.com/zh-cn/guides/search/
计费：按成功请求次数，Search / Fetch 各 5 元 / 1000 次。
"""

from __future__ import annotations

import asyncio
import ipaddress
import logging
import os
from urllib.parse import urlparse

import httpx

from config.settings import settings
from llm_relay.config import loader

logger = logging.getLogger(__name__)


class WebError(RuntimeError):
    """网页中继调用失败。"""

    def __init__(self, message: str, status: int | None = None):
        super().__init__(message)
        self.status = status


def _provider() -> dict:
    name = loader.WEB["provider"]
    provider = loader.PROVIDERS.get(name)
    if not provider:
        raise WebError(f"web.provider '{name}' 未在 providers 中定义")
    return provider


def _endpoint(path: str) -> str:
    return _provider()["base_url"].rstrip("/") + path


def _headers() -> dict[str, str]:
    key = loader.api_key(loader.WEB["provider"])
    if not key:
        raise WebError("缺少 DeepSeek API Key，请检查 .env 的 DEEPSEEK_API_KEY")
    return {"Authorization": f"Bearer {key}", "Content-Type": "application/json"}


def _proxy() -> str | None:
    env_name = loader.HTTP.get("proxy_env", "HTTP_PROXY")
    return os.getenv(env_name) or settings.http_proxy or None


def _error_message(resp: httpx.Response) -> str:
    try:
        message = (resp.json().get("error") or {}).get("message")
        if message:
            return str(message)
    except Exception:
        pass
    return f"HTTP {resp.status_code}"


async def _post(path: str, payload: dict) -> dict:
    """POST 到 DeepSeek tools 接口；408/429/5xx 退避重试，其它 4xx 直接抛。"""
    if not loader.WEB.get("enabled", True):
        raise WebError("web 中继已禁用（llm_relay/config/config.json 的 web.enabled=false）")

    timeout = httpx.Timeout(float(loader.WEB.get("timeout_seconds", 30)))
    attempts = max(1, int(loader.HTTP.get("retry_attempts", 3)))
    backoff = float(loader.HTTP.get("retry_backoff_seconds", 1.0))
    max_backoff = float(loader.HTTP.get("retry_max_backoff_seconds", 10.0))
    last_error: Exception | None = None

    async with httpx.AsyncClient(timeout=timeout, proxy=_proxy()) as client:
        for attempt in range(attempts):
            try:
                resp = await client.post(_endpoint(path), headers=_headers(), json=payload)
            except httpx.HTTPError as e:
                last_error = e
            else:
                if resp.status_code == 200:
                    return resp.json()
                if resp.status_code in (408, 429) or resp.status_code >= 500:
                    last_error = WebError(_error_message(resp), resp.status_code)
                else:
                    raise WebError(_error_message(resp), resp.status_code)
            if attempt < attempts - 1:
                await asyncio.sleep(min(backoff * (2 ** attempt), max_backoff))

    raise WebError(f"请求失败: {last_error}") from last_error


def _check_url(url: str) -> None:
    """只放行 http/https、带 host、不带凭据的 URL；私网字面量直接拒。"""
    if not url or len(url) > 8192:
        raise WebError("URL 为空或超过 8192 字符")
    try:
        parsed = urlparse(url)
    except ValueError as e:
        raise WebError(f"非法 URL: {url}") from e
    if parsed.scheme not in ("http", "https") or not parsed.hostname:
        raise WebError("URL 必须是带 host 的 http/https 地址")
    if parsed.username or parsed.password:
        raise WebError("URL 不允许携带用户名或密码")
    try:
        ip = ipaddress.ip_address(parsed.hostname)
    except ValueError:
        return
    if (
        ip.is_private
        or ip.is_loopback
        or ip.is_link_local
        or ip.is_reserved
        or ip.is_multicast
        or ip.is_unspecified
    ):
        raise WebError(f"拒绝抓取内网地址: {parsed.hostname}")


async def search(
    query: str,
    count: int | None = None,
    need_content: bool = False,
    ori_question: str | None = None,
) -> list[dict]:
    """搜索网页，返回统一后的结果列表。

    need_content=True 时 results[].content 带网页正文（计费不变）。
    """
    text = (query or "").strip()
    if not text:
        raise WebError("query 不能为空")

    max_count = int(loader.WEB.get("max_count", 50))
    n = int(count or loader.WEB.get("default_count", 5))
    n = max(1, min(n, max_count))

    payload: dict = {
        "query": text[:128],
        "count": n,
        "need_content": need_content,
    }
    if ori_question:
        payload["ori_question"] = ori_question[:128]

    data = await _post(loader.WEB["search_path"], payload)
    results = data.get("results") or []
    return [
        {
            "title": r.get("title", ""),
            "url": r.get("url", ""),
            "snippet": r.get("snippet", ""),
            "summary": r.get("summary", ""),
            "content": r.get("content", ""),
            "site_name": r.get("site_name"),
            "date": r.get("date"),
            "score": r.get("rerank_score"),
            "authority": r.get("authority"),
        }
        for r in results
    ]


async def fetch(url: str) -> dict:
    """抓取单个网页，返回 Markdown 正文。"""
    _check_url(url)
    data = await _post(loader.WEB["fetch_path"], {"url": url})
    return {
        "url": url,
        "title": data.get("title", ""),
        "content": data.get("content", ""),
    }
