"""web_fetch：抓取单个网页正文。"""

from llm_relay.llm_tools.registry import tool

from .handler import run
from .schema import PARAMETERS

tool(
    name="web_fetch",
    description="抓取指定 http/https 网页，返回标题和 Markdown 正文。",
    parameters=PARAMETERS,
    kind="web",
)(run)
