"""Agent loop 的配置。"""

from dataclasses import dataclass


@dataclass(slots=True)
class LoopConfig:
    """一次 loop 的预算和限制。"""

    max_steps: int = 8                  # 最多几次模型调用（一次模型调用算一步）
    max_tool_calls: int = 16            # 整个 task 最多执行多少个 tool_call
    max_seconds: float = 120.0          # 整个 loop 的软时间预算，在步骤之间检查
    tool_timeout_seconds: float = 30.0  # 单个工具执行的超时时间
    max_parallel_tools: int = 5         # 同一批工具里最多并行几个
    max_tool_result_chars: int = 8000   # 单个工具结果的最大字符数，超出会截断


__all__ = ["LoopConfig"]
