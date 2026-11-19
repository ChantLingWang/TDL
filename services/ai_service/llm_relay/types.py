"""llm_relay 的公共数据类型。"""

from dataclasses import dataclass, field
from typing import Literal

Role = Literal["system", "user", "assistant", "tool"]


@dataclass(slots=True)
class ToolCall:
    """模型请求调用某个工具。arguments 已解析成 dict。"""

    id: str
    name: str
    arguments: dict = field(default_factory=dict)
    raw_arguments: str = ""
    parse_error: str = ""


@dataclass(slots=True)
class ToolSpec:
    """交给模型的工具定义（JSON Schema）。"""

    name: str
    description: str
    parameters: dict = field(default_factory=dict)


@dataclass(slots=True)
class Message:
    """一条对话消息。

    assistant 带 tool_calls 表示“我要调工具”；
    role="tool" 时用 tool_call_id 对应是哪一次调用。
    """

    role: Role
    content: str = ""
    tool_calls: list[ToolCall] = field(default_factory=list)
    tool_call_id: str = ""


@dataclass(slots=True)
class LLMRequest:
    """client 解析好、交给 provider 的请求。"""

    model: str
    messages: list[Message]
    temperature: float
    max_tokens: int
    tools: list[ToolSpec] | None = None
    tool_choice: str | None = None


@dataclass(slots=True)
class Usage:
    """一次调用的 token 用量。"""

    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0


@dataclass(slots=True)
class LLMResult:
    """非流式调用的完整结果。"""

    content: str = ""
    reasoning: str = ""
    tool_calls: list[ToolCall] = field(default_factory=list)
    finish_reason: str = ""
    model: str = ""
    usage: Usage = field(default_factory=Usage)
    cost_usd: float = 0.0
