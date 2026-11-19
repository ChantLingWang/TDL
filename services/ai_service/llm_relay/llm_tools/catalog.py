"""常驻目录：注册表的 name + description 投影。

目录只放名字和一句话，不放 schema；schema 由 load_tools 按需取。
这样每轮 prompt 的成本是 O(工具数)，不是 O(工具 schema 总和)。
"""

from __future__ import annotations

from llm_relay.llm_tools.registry import ToolRegistry, registry

HEADER = (
    "## 工具目录\n"
    "以下工具可按需使用。需要某个工具时，先调用 load_tools(names=[\"工具名\"]) "
    "获取参数定义，再调用该工具；不需要就不要加载。\n"
)


def describe(reg: ToolRegistry = registry) -> dict[str, str]:
    """返回 {工具名: 描述}。"""
    return reg.describe()


def render(reg: ToolRegistry = registry, *, exclude: set[str] | None = None) -> str:
    """渲染成一段目录文本；exclude 里的工具不出现。"""
    excluded = exclude or set()
    lines = [
        f"- {item.name}：{item.description}"
        for item in sorted(reg.all(), key=lambda item: item.name)
        if item.name not in excluded
    ]
    if not lines:
        return HEADER + "\n（没有其他可加载的工具）"
    return HEADER + "\n" + "\n".join(lines)


__all__ = ["HEADER", "describe", "render"]
