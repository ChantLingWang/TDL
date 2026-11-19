"""按需加载：Toolbox 持有已加载集合，load_tools 是模型可见的入口。

模型每轮只看到常驻工具的 schema（目前只有 load_tools）和一段目录；
需要别的工具时先调用 load_tools 拿 schema，它同时把工具放进
Toolbox.loaded，下一轮 specs() 就会带上，模型才能真的调用。
"""

from __future__ import annotations

import asyncio
import inspect
import json
import logging
from typing import Any

from llm_relay.llm_tools import catalog as catalog_module
from llm_relay.llm_tools.registry import (
    Tool,
    ToolContext,
    ToolRegistry,
    registry as default_registry,
    tool,
)
from llm_relay.types import ToolSpec

logger = logging.getLogger(__name__)

MAX_NAMES = 20
"""一次 load_tools 最多加载多少个，防止返回体过大。"""


class Toolbox:
    """一次会话 / 一次请求的工具视图。

    specs()   = 常驻工具 + 已加载工具，直接传给 llm.complete(tools=...)
    catalog() = 还没加载的工具目录，拼进 system prompt
    """

    def __init__(
        self,
        registry: ToolRegistry | None = None,
        ctx: ToolContext | None = None,
    ) -> None:
        self.registry = registry or default_registry
        self.ctx = ctx or ToolContext()
        self.ctx.toolbox = self
        self.loaded: set[str] = set()

    def specs(self) -> list[ToolSpec]:
        return self.registry.specs(sorted(self.loaded))

    def catalog(self) -> str:
        exclude = {item.name for item in self.registry.resident()} | self.loaded
        return catalog_module.render(self.registry, exclude=exclude)

    def activate(self, names: list[str]) -> tuple[list[Tool], list[str], list[str]]:
        """把工具加入已加载集合，返回 (新加载, 已可用, 未知)。"""
        loaded: list[Tool] = []
        already: list[str] = []
        unknown: list[str] = []
        for name in dict.fromkeys(names):
            item = self.registry.get(name)
            if item is None:
                unknown.append(name)
            elif item.resident or name in self.loaded:
                already.append(name)
            else:
                self.loaded.add(name)
                loaded.append(item)
        return loaded, already, unknown

    async def call(self, name: str, arguments: dict | None = None) -> str:
        """执行一次工具调用，始终返回字符串（供 tool message 使用）。"""
        item = self.registry.get(name)
        if item is None:
            return _error(f"未知工具: {name}，请先调用 load_tools 加载")
        if not item.resident and name not in self.loaded:
            return _error(f"工具 {name} 尚未加载，请先调用 load_tools")

        args = {} if arguments is None else arguments
        if not isinstance(args, dict):
            return _error(f"工具 {name} 的参数必须是 JSON 对象")

        try:
            if inspect.iscoroutinefunction(item.handler):
                result = await item.handler(self.ctx, **args)
            else:
                result = await asyncio.to_thread(item.handler, self.ctx, **args)
                if inspect.isawaitable(result):
                    result = await result
        except Exception as exc:
            logger.exception("工具执行失败 name=%s", name)
            return _error(f"工具 {name} 执行失败: {exc}")

        return _dump(result)


@tool(
    name="load_tools",
    description=(
        "按工具名加载参数定义（JSON Schema）。需要某个工具时先调用它，"
        "返回后该工具即可在后续轮次调用；不需要的工具不要加载。"
    ),
    parameters={
        "type": "object",
        "properties": {
            "names": {
                "type": "array",
                "items": {"type": "string"},
                "description": "要加载的工具名列表",
            }
        },
        "required": ["names"],
        "additionalProperties": False,
    },
    kind="local",
    resident=True,
)
async def load_tools(ctx: ToolContext, names: list[str]) -> str:
    box = getattr(ctx, "toolbox", None)
    if box is None:
        return _error("load_tools 缺少 Toolbox 上下文")
    if not isinstance(names, list) or not all(isinstance(n, str) for n in names):
        return _error("names 必须是字符串数组")
    if not names:
        return _error("names 不能为空")
    if len(names) > MAX_NAMES:
        return _error(f"一次最多加载 {MAX_NAMES} 个工具")

    loaded, already, unknown = box.activate(names)
    return json.dumps(
        {
            "loaded": [
                {
                    "name": item.name,
                    "description": item.description,
                    "parameters": item.parameters,
                }
                for item in loaded
            ],
            "already": already,
            "unknown": unknown,
        },
        ensure_ascii=False,
    )


def _error(message: str) -> str:
    return json.dumps({"error": message}, ensure_ascii=False)


def _dump(value: Any) -> str:
    if value is None:
        return ""
    if isinstance(value, str):
        return value
    try:
        return json.dumps(value, ensure_ascii=False, default=str)
    except TypeError:
        return str(value)


__all__ = ["Toolbox", "load_tools"]
