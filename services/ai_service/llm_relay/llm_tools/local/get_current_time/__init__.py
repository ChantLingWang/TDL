"""get_current_time：返回当前北京时间。"""

from llm_relay.llm_tools.registry import tool

from .handler import run
from .schema import PARAMETERS

tool(
    name="get_current_time",
    description="返回当前北京时间；默认 ISO 8601，可切换为中文可读格式。",
    parameters=PARAMETERS,
    kind="local",
)(run)
