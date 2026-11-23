"""Agent loop：模型决策 -> 工具执行 -> 再决策，直到最终回答。"""

from __future__ import annotations

import json
import time
from collections.abc import Awaitable, Callable

from llm_relay.client import LLMClient
from llm_relay.llm_tools.loader import Toolbox
from llm_relay.types import Message, ToolCall, Usage
from runtime.agent_loop.executor import execute_tool_batch
from runtime.agent_loop.config import LoopConfig
from runtime.agent_loop.schemas import (
    LoopResult,
    Step,
    StepEvent,
    StopReason,
    ToolResult,
)

FORCE_FINAL_PROMPT = "已达到工具/时间上限，请基于现有信息直接回答，不要调用工具。"

StepCallback = Callable[[StepEvent], Awaitable[None]]


class _StepEmitter:
    """分配 step 号并把事件交给调用方；没有回调时只计数。"""

    def __init__(self, callback: StepCallback | None) -> None:
        self._callback = callback
        self.step = 0

    async def emit(self, kind: str, payload: dict) -> None:
        self.step += 1
        if self._callback is None:
            return
        await self._callback(StepEvent(step=self.step, kind=kind, payload=payload))


class AgentLoop:
    """一次任务的 agent 循环；不感知 Kafka、DB、前端。"""

    def __init__(
        self,
        llm: LLMClient,
        toolbox: Toolbox,
        config: LoopConfig | None = None,
        on_step: StepCallback | None = None,
    ) -> None:
        self._llm = llm
        self._toolbox = toolbox
        self._config = config or LoopConfig()
        self._on_step = on_step

    async def run(self, messages: list[Message], *, task: str = "chat") -> LoopResult:
        """跑完一次任务，返回最终文本和完整步骤。

        messages 由调用方组装好（system/history/current）；loop 只追加
        assistant/tool 消息，并复制一份 list，不修改调用方传入的 list。
        """
        messages = list(messages)
        steps: list[Step] = []
        seq = 0
        total_tool_calls = 0
        usage = Usage()
        cost_usd = 0.0
        model = ""
        deadline = time.monotonic() + self._config.max_seconds
        emitter = _StepEmitter(self._on_step)

        for _ in range(max(1, self._config.max_steps)):
            # 软时间预算：只在每次模型调用前检查，不打断进行中的请求。
            if time.monotonic() >= deadline:
                return await self._force_final(
                    messages,
                    task=task,
                    steps=steps,
                    seq=seq,
                    usage=usage,
                    cost_usd=cost_usd,
                    model=model,
                    stop_reason="timeout",
                    emitter=emitter,
                )

            # 工具 schema 会随 load_tools 变化，所以每轮都重新取 specs()。
            result = await self._llm.complete(
                messages,
                task=task,
                tools=self._toolbox.specs(),
            )
            _merge_usage(usage, result.usage)
            cost_usd += result.cost_usd
            model = result.model or model

            if result.reasoning:
                await emitter.emit("THINK", {"think": result.reasoning})

            tool_calls = result.tool_calls or []
            assistant = Step(
                seq=seq,
                kind="assistant",
                reasoning=result.reasoning,
                content=result.content,
                tool_calls=list(tool_calls),
                usage=result.usage,
                finish_reason=result.finish_reason,
            )
            steps.append(assistant)
            seq += 1

            # 没有 tool_calls 就是最终回答，直接结束。
            if not tool_calls:
                status = "completed" if result.content else "failed"
                reason = "" if result.content else "empty reply"
                await emitter.emit(
                    "RESULT",
                    {"result": result.content, "status": status, "reason": reason},
                )
                return LoopResult(
                    text=result.content,
                    reasoning=result.reasoning,
                    usage=usage,
                    cost_usd=cost_usd,
                    model=model,
                    steps=steps,
                    stop_reason="final",
                )

            await emitter.emit(
                "TOOL_CALL",
                {"calls": [_tool_call_payload(call) for call in tool_calls]},
            )

            # 先把 assistant 的 tool_calls 追加进历史，再追 tool 结果。
            messages.append(
                Message(
                    role="assistant",
                    content=result.content,
                    tool_calls=list(tool_calls),
                )
            )

            # 一批工具统一执行；结果仍按模型返回的顺序写回。
            tool_results = await execute_tool_batch(
                self._toolbox, list(tool_calls), self._config
            )
            await emitter.emit(
                "TOOL_RESULT",
                {"results": [_tool_result_payload(item) for item in tool_results]},
            )
            steps.append(
                Step(
                    seq=seq,
                    kind="tool_batch",
                    parent_seq=assistant.seq,
                    results=tool_results,
                )
            )
            seq += 1

            # 每个 tool_call 都必须有一条 tool message，参数配对靠 tool_call_id。
            for item in tool_results:
                messages.append(
                    Message(
                        role="tool",
                        content=item.result,
                        tool_call_id=item.tool_call_id,
                    )
                )

            total_tool_calls += len(tool_calls)
            # 超出总工具预算后不再让模型继续调工具，直接收口。
            if total_tool_calls > self._config.max_tool_calls:
                return await self._force_final(
                    messages,
                    task=task,
                    steps=steps,
                    seq=seq,
                    usage=usage,
                    cost_usd=cost_usd,
                    model=model,
                    stop_reason="max_tool_calls",
                    emitter=emitter,
                )

        # 步数用完，强制模型基于现有信息给最终回答。
        return await self._force_final(
            messages,
            task=task,
            steps=steps,
            seq=seq,
            usage=usage,
            cost_usd=cost_usd,
            model=model,
            stop_reason="max_steps",
            emitter=emitter,
        )

    async def _force_final(
        self,
        messages: list[Message],
        *,
        task: str,
        steps: list[Step],
        seq: int,
        usage: Usage,
        cost_usd: float,
        model: str,
        stop_reason: StopReason,
        emitter: _StepEmitter,
    ) -> LoopResult:
        """预算耗尽时，追加提示并做一次无工具的模型调用，拿到最终回答。"""
        messages.append(Message(role="user", content=FORCE_FINAL_PROMPT))
        result = await self._llm.complete(messages, task=task, tools=None)
        _merge_usage(usage, result.usage)
        cost_usd += result.cost_usd
        model = result.model or model

        if result.reasoning:
            await emitter.emit("THINK", {"think": result.reasoning})

        status = "completed" if result.content else "failed"
        reason = "" if result.content else stop_reason
        await emitter.emit(
            "RESULT",
            {"result": result.content, "status": status, "reason": reason},
        )

        steps.append(
            Step(
                seq=seq,
                kind="assistant",
                reasoning=result.reasoning,
                content=result.content,
                usage=result.usage,
                finish_reason=result.finish_reason,
            )
        )
        return LoopResult(
            text=result.content,
            reasoning=result.reasoning,
            usage=usage,
            cost_usd=cost_usd,
            model=model,
            steps=steps,
            stop_reason=stop_reason,
        )


def _tool_call_payload(call: ToolCall) -> dict:
    """TOOL_CALL 里单个工具调用的 JSON。"""
    item: dict = {
        "id": call.id,
        "name": call.name,
        "input": call.arguments,
    }
    if call.parse_error:
        item["error"] = call.parse_error
    return item


def _tool_result_payload(item: ToolResult) -> dict:
    """TOOL_RESULT 里单个工具返回的 JSON。"""
    payload = _parse_json(item.result)
    entry: dict = {
        "id": item.tool_call_id,
        "name": item.name,
        "payload": payload,
        "status": item.status,
        "duration_ms": item.duration_ms,
    }
    if item.status != "completed":
        entry["error"] = item.result
    return entry


def _parse_json(text: str):
    """工具返回是 JSON 就还原成对象/数组，否则保留字符串。"""
    try:
        return json.loads(text)
    except (TypeError, json.JSONDecodeError):
        return text


def _merge_usage(total: Usage, part: Usage | None) -> None:
    """把一次模型调用的 usage 累加到总 usage。"""
    if part is None:
        return
    total.prompt_tokens += part.prompt_tokens
    total.completion_tokens += part.completion_tokens
    total.total_tokens += part.total_tokens


__all__ = ["AgentLoop"]
