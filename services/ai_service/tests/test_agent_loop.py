"""runtime.agent_loop 测试：真实 Toolbox + 脚本化 LLM，不触网。"""

import asyncio
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from llm_relay.llm_tools.loader import Toolbox, load_tools as load_tools_handler  # noqa: E402
from llm_relay.llm_tools.registry import Tool, ToolRegistry  # noqa: E402
from llm_relay.types import LLMResult, Message, ToolCall, Usage  # noqa: E402
from runtime.agent_loop import AgentLoop, LoopConfig  # noqa: E402


class ScriptedLLM:
    """按预设顺序返回 LLMResult，并记录每次请求的 messages。"""

    def __init__(self, results):
        self._results = list(results)
        self.calls = []

    async def complete(self, messages, **kwargs):
        self.calls.append((list(messages), dict(kwargs)))
        if not self._results:
            raise AssertionError("ScriptedLLM 没有更多结果")
        return self._results.pop(0)


def _result(
    content="",
    tool_calls=None,
    reasoning="",
    usage=None,
    cost=0.0,
    model="test",
):
    calls = tool_calls or []
    return LLMResult(
        content=content,
        reasoning=reasoning,
        tool_calls=calls,
        finish_reason="tool_calls" if calls else "stop",
        model=model,
        usage=usage or Usage(),
        cost_usd=cost,
    )


def _tool(name, handler, *, resident=False):
    return Tool(
        name=name,
        description=name,
        handler=handler,
        parameters={"type": "object", "properties": {}},
        resident=resident,
    )


def _box(*tools, active=()):
    reg = ToolRegistry()
    for item in tools:
        reg.register(item)
    box = Toolbox(registry=reg)
    for name in active:
        box.activate([name])
    return box


def _run(llm, box, config=None):
    return asyncio.run(
        AgentLoop(llm, box, config).run([Message(role="user", content="hi")])
    )


def test_final_without_tools():
    box = _box()
    llm = ScriptedLLM(
        [_result(content="hello", usage=Usage(10, 2, 12), cost=0.01, model="m")]
    )
    res = _run(llm, box)

    assert res.stop_reason == "final"
    assert res.text == "hello"
    assert len(res.steps) == 1
    assert res.steps[0].kind == "assistant"
    assert res.usage.total_tokens == 12
    assert res.cost_usd == 0.01
    assert res.model == "m"
    assert llm.calls[0][1]["tools"] == []


def test_one_tool_then_final():
    async def echo(ctx, text=""):
        return f"echo:{text}"

    box = _box(_tool("echo", echo), active=("echo",))
    call = ToolCall(id="c1", name="echo", arguments={"text": "hi"})
    llm = ScriptedLLM(
        [
            _result(tool_calls=[call], usage=Usage(10, 2, 12), cost=0.01),
            _result(content="done", usage=Usage(3, 1, 4), cost=0.02),
        ]
    )
    res = _run(llm, box)

    assert [step.kind for step in res.steps] == [
        "assistant",
        "tool_batch",
        "assistant",
    ]
    assert res.steps[1].parent_seq == 0
    assert res.steps[1].results[0].status == "completed"
    assert res.steps[1].results[0].result == "echo:hi"
    assert res.steps[2].seq == 2
    assert res.text == "done"
    assert res.usage.total_tokens == 16
    assert round(res.cost_usd, 6) == 0.03

    second = llm.calls[1][0]
    assert [message.role for message in second] == ["user", "assistant", "tool"]
    assert second[1].tool_calls[0].id == "c1"
    assert second[2].tool_call_id == "c1"
    assert second[2].content == "echo:hi"


def test_multiple_tools_parallel():
    active = 0
    peak = 0

    async def tracked(ctx, name=""):
        nonlocal active, peak
        active += 1
        peak = max(peak, active)
        await asyncio.sleep(0.02)
        active -= 1
        return name

    box = _box(_tool("tracked", tracked), active=("tracked",))
    calls = [
        ToolCall(id=f"c{index}", name="tracked", arguments={"name": str(index)})
        for index in range(3)
    ]
    llm = ScriptedLLM([_result(tool_calls=calls), _result(content="done")])
    res = _run(llm, box, LoopConfig(max_parallel_tools=3))

    batch = res.steps[1]
    assert batch.kind == "tool_batch"
    assert [item.result for item in batch.results] == ["0", "1", "2"]
    assert all(item.status == "completed" for item in batch.results)
    assert peak >= 2

    second = llm.calls[1][0]
    tool_messages = [message for message in second if message.role == "tool"]
    assert [message.tool_call_id for message in tool_messages] == ["c0", "c1", "c2"]


def test_unknown_tool_becomes_result():
    box = _box()
    call = ToolCall(id="x", name="nope", arguments={})
    llm = ScriptedLLM([_result(tool_calls=[call]), _result(content="recovered")])
    res = _run(llm, box)

    assert res.steps[1].results[0].status == "failed"
    assert "未知工具" in res.steps[1].results[0].result
    assert res.text == "recovered"


def test_tool_exception_becomes_result():
    async def boom(ctx):
        raise RuntimeError("boom")

    box = _box(_tool("boom", boom), active=("boom",))
    call = ToolCall(id="c1", name="boom", arguments={})
    llm = ScriptedLLM([_result(tool_calls=[call]), _result(content="recovered")])
    res = _run(llm, box)

    assert res.steps[1].results[0].status == "failed"
    assert "boom" in res.steps[1].results[0].result
    assert res.text == "recovered"


def test_tool_timeout():
    async def slow(ctx):
        await asyncio.sleep(0.1)
        return "late"

    box = _box(_tool("slow", slow), active=("slow",))
    call = ToolCall(id="c1", name="slow", arguments={})
    llm = ScriptedLLM([_result(tool_calls=[call]), _result(content="recovered")])
    res = _run(llm, box, LoopConfig(tool_timeout_seconds=0.01))

    assert res.steps[1].results[0].status == "timeout"
    assert "超时" in res.steps[1].results[0].result
    assert res.text == "recovered"


def test_load_tools_barrier():
    async def echo(ctx, text=""):
        return f"echo:{text}"

    reg = ToolRegistry()
    reg.register(_tool("echo", echo))
    reg.register(
        Tool(
            name="load_tools",
            description="load tools",
            handler=load_tools_handler,
            parameters={"type": "object", "properties": {}},
            resident=True,
        )
    )
    box = Toolbox(registry=reg)
    calls = [
        ToolCall(id="c1", name="load_tools", arguments={"names": ["echo"]}),
        ToolCall(id="c2", name="echo", arguments={"text": "hi"}),
    ]
    llm = ScriptedLLM([_result(tool_calls=calls), _result(content="done")])
    res = _run(llm, box)

    assert [item.status for item in res.steps[1].results] == [
        "completed",
        "completed",
    ]
    assert "echo" in box.loaded
    assert res.steps[1].results[1].result == "echo:hi"
    assert res.text == "done"


def test_max_steps_forces_final():
    async def echo(ctx):
        return "ok"

    box = _box(_tool("echo", echo), active=("echo",))
    call = ToolCall(id="c1", name="echo", arguments={})
    llm = ScriptedLLM([_result(tool_calls=[call]), _result(content="forced")])
    res = _run(llm, box, LoopConfig(max_steps=1))

    assert res.stop_reason == "max_steps"
    assert res.text == "forced"
    assert [step.kind for step in res.steps] == [
        "assistant",
        "tool_batch",
        "assistant",
    ]
    assert res.steps[-1].seq == 2


def test_max_tool_calls_forces_final():
    async def echo(ctx):
        return "ok"

    box = _box(_tool("echo", echo), active=("echo",))
    calls = [
        ToolCall(id=f"c{index}", name="echo", arguments={}) for index in range(2)
    ]
    llm = ScriptedLLM([_result(tool_calls=calls), _result(content="forced")])
    res = _run(llm, box, LoopConfig(max_tool_calls=1))

    assert res.stop_reason == "max_tool_calls"
    assert len(res.steps[1].results) == 2
    assert res.text == "forced"


def test_max_seconds_forces_final():
    box = _box()
    llm = ScriptedLLM([_result(content="forced")])
    res = _run(llm, box, LoopConfig(max_seconds=0))

    assert res.stop_reason == "timeout"
    assert res.text == "forced"
    assert len(llm.calls) == 1


def test_tool_result_truncated():
    async def big(ctx):
        return "x" * 100

    box = _box(_tool("big", big), active=("big",))
    call = ToolCall(id="c1", name="big", arguments={})
    llm = ScriptedLLM([_result(tool_calls=[call]), _result(content="done")])
    res = _run(llm, box, LoopConfig(max_tool_result_chars=40))

    output = res.steps[1].results[0].result
    assert "已截断" in output
    assert len(output) <= 40
