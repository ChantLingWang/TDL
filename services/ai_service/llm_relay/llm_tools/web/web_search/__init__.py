"""web_search：互联网搜索。"""

from llm_relay.llm_tools.registry import tool

from .handler import run
from .schema import PARAMETERS

tool(
    name="web_search",
    description="搜索互联网，返回相关网页的标题、链接、摘要；可选返回正文。",
    parameters=PARAMETERS,
    kind="web",
)(run)
