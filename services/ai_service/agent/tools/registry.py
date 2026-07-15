"""工具注册表 —— 加载通用工具并合并为 TOOL_MAP。"""

TOOL_MAP: dict = {}


def register_tools(tools: dict) -> None:
    """注册额外工具到全局 TOOL_MAP。"""
    TOOL_MAP.update(tools)


def load_default_tools() -> None:
    """加载通用工具。后续在此 import 各工具模块。"""
    pass
