"""工具注册表：name -> Tool 的唯一真相。

工具在 import 时通过 @tool 注册；注册表只做登记和查询，
不关心工具是 web（HTTP）还是 local（本地函数），也不执行。

目录（catalog）和按需加载（load_tools）都从这里取数，
所以新增工具只需要新增一个包并在 llm_tools/__init__.py 里 import，
不需要改注册表、目录或 loader。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Callable, Iterable, Literal

from llm_relay.types import ToolSpec

ToolKind = Literal["web", "local"]
Handler = Callable[..., Any]


@dataclass(slots=True, frozen=True)
class Tool:
    """一个模型可调用的工具。"""

    name: str
    description: str
    handler: Handler
    parameters: dict = field(default_factory=lambda: {"type": "object", "properties": {}})
    kind: ToolKind = "local"
    resident: bool = False
    """True 表示每轮都直接带 schema；False 表示要 load_tools 后才可见。"""

    def spec(self) -> ToolSpec:
        """转成 llm_relay 交给模型的 ToolSpec。"""
        return ToolSpec(
            name=self.name,
            description=self.description,
            parameters=self.parameters,
        )


@dataclass(slots=True)
class ToolContext:
    """一次工具执行能拿到的会话信息。"""

    user_id: str = ""
    conversation_id: str = ""
    message_id: str = ""
    toolbox: Any = None
    """由 Toolbox 注入；load_tools 用它登记已加载工具。"""


class ToolRegistry:
    """name -> Tool 的注册表；重名在 import 阶段就失败。"""

    def __init__(self) -> None:
        self._tools: dict[str, Tool] = {}

    def register(self, tool: Tool) -> Tool:
        if not tool.name:
            raise ValueError("工具 name 不能为空")
        if not tool.description:
            raise ValueError(f"工具 {tool.name} 缺少 description")
        if not isinstance(tool.parameters, dict):
            raise TypeError(f"工具 {tool.name} 的 parameters 必须是 dict")

        existing = self._tools.get(tool.name)
        if existing is not None:
            raise ValueError(
                f"工具重名: {tool.name}（已由 {_module(existing)} 注册，"
                f"又来自 {_module(tool)}）"
            )

        self._tools[tool.name] = tool
        return tool

    def get(self, name: str) -> Tool | None:
        return self._tools.get(name)

    def all(self) -> list[Tool]:
        return list(self._tools.values())

    def resident(self) -> list[Tool]:
        """每轮无条件带给模型的工具。"""
        return [item for item in self._tools.values() if item.resident]

    def describe(self) -> dict[str, str]:
        """返回 {工具名: 描述}，即常驻目录的数据形态。"""
        return {item.name: item.description for item in self._tools.values()}

    def specs(self, names: Iterable[str] = ()) -> list[ToolSpec]:
        """常驻工具 + names 里已注册的工具，去重后转成 ToolSpec。"""
        chosen = self.resident() + [self._tools[n] for n in names if n in self._tools]
        seen: set[str] = set()
        specs: list[ToolSpec] = []
        for item in chosen:
            if item.name in seen:
                continue
            seen.add(item.name)
            specs.append(item.spec())
        return specs

    def clear(self) -> None:
        """清空注册表；仅供测试隔离使用。"""
        self._tools.clear()


def _module(tool: Tool) -> str:
    return getattr(tool.handler, "__module__", "?")


registry = ToolRegistry()
"""进程级默认注册表；工具包 import 时写这里。"""


def tool(
    *,
    name: str,
    description: str,
    parameters: dict | None = None,
    kind: ToolKind = "local",
    resident: bool = False,
):
    """把函数登记为工具，函数本身保持不变。

    用法：
        @tool(name="get_current_time", description="...", parameters={...})
        async def get_current_time(ctx, ...): ...
    """

    def decorate(fn: Handler) -> Handler:
        registry.register(
            Tool(
                name=name,
                description=description,
                handler=fn,
                parameters=parameters or {"type": "object", "properties": {}},
                kind=kind,
                resident=resident,
            )
        )
        return fn

    return decorate


__all__ = [
    "Handler",
    "Tool",
    "ToolContext",
    "ToolKind",
    "ToolRegistry",
    "registry",
    "tool",
]
