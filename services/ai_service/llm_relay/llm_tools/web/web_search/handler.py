"""web_search 的执行函数。"""

from llm_relay.llm_tools.web.client import search


async def run(
    ctx, query: str, count: int | None = None, need_content: bool = False
) -> list[dict]:
    """搜索网页，返回统一后的结果列表。"""
    return await search(query, count=count, need_content=need_content)
