"""LLM 抽象基类和数据模型。

所有模型 provider 必须实现此接口，保证 chat/agent 模块与具体模型解耦。

扩展方式（两步）：
    1. providers/ 下新建文件，继承 AbstractLLM
    2. 用 @register("名称") 装饰器注册
    3. 在 factory.py 中 import 该文件（触发注册）

换模型只需修改 .env 中的 LLM_PROVIDER。
"""

from abc import ABC, abstractmethod
from collections.abc import AsyncIterator
from dataclasses import dataclass, field


@dataclass
class LLMMessage:
    """LLM 对话中的一条消息，对应 OpenAI messages 数组中的一项。"""
    role: str       # "system" | "user" | "assistant"
    content: str


@dataclass
class LLMResponse:
    """LLM 单次 chat 调用返回结果。"""
    content: str    # 回复文本
    model: str = "" # 实际使用的模型名
    usage: dict = field(default_factory=dict)  # token 用量信息


class AbstractLLM(ABC):
    """LLM 抽象基类。

    两个核心方法：
        chat()         —— 完整回复，等待 LLM 返回后一次性给出
        chat_stream()  —— 流式输出，逐 token 异步迭代（v1 暂不使用，预留）

    子类只需实现这两个方法，外部通过 factory.get_llm() 获取实例。
    """

    @abstractmethod
    async def chat(
        self, messages: list[LLMMessage], **kwargs
    ) -> LLMResponse:
        """非流式对话，返回完整回复。"""
        ...

    @abstractmethod
    async def chat_stream(
        self, messages: list[LLMMessage], **kwargs
    ) -> AsyncIterator[str]:
        """流式对话，逐 token 产出文本。"""
        ...
