"""web_fetch 的执行函数。"""

from llm_relay.llm_tools.web.client import fetch


async def run(ctx, url: str) -> dict:
    """抓取网页，返回 {url, title, content}。"""
    return await fetch(url)
