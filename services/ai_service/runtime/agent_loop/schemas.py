"""Agent loop 的数据结构。"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Literal

from llm_relay.types import ToolCall, Usage

StepKind = Literal["assistant", "tool_batch"]          # 模型调用 / 工具批次
ToolStatus = Literal["completed", "failed", "timeout"]  # 单个工具的执行状态
StopReason = Literal[
    "final", "max_steps", "max_tool_calls", "timeout", "error"
]  # loop 为什么结束


@dataclass(slots=True)
class ToolResult:
    """一个 tool_call 的执行结果。"""

    tool_call_id: str    # 对应 assistant 消息里的 tool_call.id
    name: str            # 工具名
    arguments: dict      # 模型传入的参数
    status: ToolStatus   # completed / failed / timeout
    result: str          # 工具返回文本；错误信息也放这里
    duration_ms: int     # 执行耗时，毫秒


@dataclass(slots=True)
class Step:
    """一次循环步骤：模型调用或工具批次。"""

    seq: int                      # task 内单调递增
    kind: StepKind                # assistant / tool_batch
    parent_seq: int | None = None  # tool_batch 指向发起它的 assistant step
    reasoning: str = ""            # assistant 的思考内容
    content: str = ""              # assistant 的正文
    tool_calls: list[ToolCall] = field(default_factory=list)  # assistant 请求的工具
    results: list[ToolResult] = field(default_factory=list)   # tool_batch 的执行结果
    usage: Usage | None = None    # 仅 assistant step 有 token 用量
    finish_reason: str = ""       # 模型返回的 finish_reason


@dataclass(slots=True)
class LoopResult:
    """一次 loop 的最终结果。"""

    text: str                 # 最终回答
    reasoning: str            # 最终一次模型调用的思考
    usage: Usage              # 整个 task 的累计 token
    cost_usd: float           # 整个 task 的累计成本
    model: str                # 最后一次使用的模型
    steps: list[Step]         # 完整过程，后续可交给 Kafka / 持久化
    stop_reason: StopReason   # 为什么结束


__all__ = [
    "LoopResult",
    "Step",
    "StepKind",
    "StopReason",
    "ToolResult",
    "ToolStatus",
]
