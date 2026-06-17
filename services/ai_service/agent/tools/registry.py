"""工具注册表。加载所有工具并合并为 TOOL_MAP。"""

TOOL_MAP: dict = {}


def register_tools(tools: dict) -> None:
    """注册额外工具到全局 TOOL_MAP。"""
    TOOL_MAP.update(tools)


def load_default_tools() -> None:
    """加载通用工具。目前为空，后续在此 import。"""
    pass


def load_skill_tools(skill: str) -> None:
    """根据 skill 名称加载专属工具。"""
    # 后续按 skill 动态 import
    pass
