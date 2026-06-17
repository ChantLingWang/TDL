"""模型路由 / 编排层。

当前 v1：直接委托给 factory.get_llm() 的单例。
未来扩展：按任务类型选模型、多模型投票、agent 链中步骤级切换。
"""

from shared.llm.base import AbstractLLM, LLMMessage, LLMResponse
from shared.llm.factory import get_llm


async def route_chat(messages: list[LLMMessage], **kwargs) -> LLMResponse:
    llm = get_llm()
    return await llm.chat(messages, **kwargs)
