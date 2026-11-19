"""工具批次执行：load_tools 屏障 + 其余工具并行。"""

from __future__ import annotations

import asyncio
import json
import time

from llm_relay.llm_tools.loader import Toolbox
from llm_relay.types import ToolCall
from runtime.agent_loop.config import LoopConfig
from runtime.agent_loop.schemas import ToolResult, ToolStatus


async def execute_tool_batch(
    toolbox: Toolbox,
    calls: list[ToolCall],
    config: LoopConfig,
) -> list[ToolResult]:
    """执行一批 tool_call，返回与 calls 顺序一致的结果。

    load_tools 会改变 Toolbox 的已加载集合，所以必须先串行执行；
    其余工具用 Semaphore 限制并发，结果回填到原始下标，保证顺序。
    """
    if not calls:
        return []

    # 先占位，后面不管哪个工具先完成，都按 calls 的原始顺序返回。
    slots: list[ToolResult | None] = [None] * len(calls)

    # load_tools 是屏障：它没执行完，同批其他工具可能还没被加载。
    load_indices = [
        index for index, call in enumerate(calls) if call.name == "load_tools"
    ]
    for index in load_indices:
        slots[index] = await _execute_one(toolbox, calls[index], config)

    load_set = set(load_indices)
    others = [index for index in range(len(calls)) if index not in load_set]
    if others:
        # 并行只发生在非 load_tools 的调用上，并发上限来自 LoopConfig。
        semaphore = asyncio.Semaphore(max(1, config.max_parallel_tools))

        async def run(index: int) -> None:
            async with semaphore:
                slots[index] = await _execute_one(toolbox, calls[index], config)

        await asyncio.gather(*(run(index) for index in others))

    return [item for item in slots if item is not None]


async def _execute_one(
    toolbox: Toolbox,
    call: ToolCall,
    config: LoopConfig,
) -> ToolResult:
    """执行单个工具，把异常和超时都收敛成 ToolResult。

    Toolbox.call 自己会捕获 handler 异常并返回 JSON error；
    这里再包一层 wait_for，负责超时和统一状态。
    """
    started = time.monotonic()
    try:
        timeout = config.tool_timeout_seconds
        if timeout and timeout > 0:
            raw = await asyncio.wait_for(
                toolbox.call(call.name, call.arguments), timeout=timeout
            )
        else:
            raw = await toolbox.call(call.name, call.arguments)

        # Toolbox.call 用 {"error": ...} 表示失败；其他都算成功。
        status: ToolStatus = "failed" if _is_error(raw) else "completed"
        result = _truncate(raw, config.max_tool_result_chars)
    except asyncio.TimeoutError:
        status = "timeout"
        result = _error(f"工具 {call.name} 超时")
    except Exception as exc:
        status = "failed"
        result = _error(f"工具 {call.name} 执行失败: {exc}")

    duration_ms = int((time.monotonic() - started) * 1000)
    return ToolResult(
        tool_call_id=call.id,
        name=call.name,
        arguments=call.arguments,
        status=status,
        result=result,
        duration_ms=duration_ms,
    )


def _error(message: str) -> str:
    """构造给模型看的 JSON 错误文本。"""
    return json.dumps({"error": message}, ensure_ascii=False)


def _is_error(text: str) -> bool:
    """判断 Toolbox.call 返回的是不是错误 JSON。"""
    try:
        data = json.loads(text)
    except (TypeError, json.JSONDecodeError):
        return False
    return isinstance(data, dict) and "error" in data


def _truncate(text: str, limit: int) -> str:
    """超长结果保留头尾，中间用标记省略。"""
    if limit <= 0 or len(text) <= limit:
        return text
    marker = f"\n...（已截断 {len(text) - limit} 字符）...\n"
    keep = max(0, limit - len(marker))
    head = keep * 2 // 3
    tail = keep - head
    return text[:head] + marker + (text[-tail:] if tail else "")


__all__ = ["execute_tool_batch"]
