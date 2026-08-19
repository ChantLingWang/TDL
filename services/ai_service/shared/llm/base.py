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
    reasoning: str = ""  # 思考链文本（模型支持思考时才有）
    model: str = "" # 实际使用的模型名
    usage: dict = field(default_factory=dict)  # token 用量信息


@dataclass
class LLMStreamChunk:
    """chat_stream 产出的单个流式增量块。

    kind 取值：
        "reasoning"  思考链文本增量
        "content"    正文文本增量
    usage 仅在流末尾的结算块携带（text 为空），其余块为空字典。
    """
    kind: str   # "reasoning" | "content"
    text: str
    usage: dict = field(default_factory=dict)


class AbstractLLM(ABC):
    """LLM 抽象基类。

    核心方法：
        chat()            —— 完整回复，等待 LLM 返回后一次性给出
        chat_stream()     —— 流式输出，逐 token 异步迭代（v1 暂不使用，预留）
        langchain_model() —— 返回 langchain ChatOpenAI 实例，给 agent 图使用
        get_pricing()     —— 返回指定模型的 (input, output) 定价

    子类需实现这四个方法，外部通过 factory.get_llm() 获取实例。
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
    ) -> AsyncIterator[LLMStreamChunk]:
        """流式对话，逐块产出（reasoning / content 分流）。

        流末尾会额外产出一个带 usage 的空块用于成本结算。
        """
        ...
    @abstractmethod
    def langchain_model(
        self, temperature: float = 0.3, max_tokens: int | None = None
    ):
        """Return a langchain ChatOpenAI instance configured with this provider's
        api_key, base_url, model, and proxy settings.

        The agent research graph uses langchain's .ainvoke / .bind interface;
        this method keeps provider dispatch centralized in the LLM abstraction layer.
        """
        ...

    @abstractmethod
    def get_pricing(self, model: str | None = None) -> dict[str, float]:
        """Return pricing dict {"input": 0.27, "output": 1.10} in USD per million tokens."""
        ...

