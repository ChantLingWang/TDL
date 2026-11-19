"""LLM 统一入口。

只提供非流式调用：一次请求拿到完整 content / tool_calls / usage / cost。
工具轮和最终回答都走 complete()，由 runtime.agent_loop 决定下一步。

用法：
    from llm_relay.client import llm
    from llm_relay.types import Message, ToolSpec

    result = await llm.complete(messages, tools=[...])
"""

from __future__ import annotations

import asyncio

from llm_relay.config import loader
from llm_relay.errors import LLMError
from llm_relay.providers.chat_completions import ChatCompletionsProvider
from llm_relay.types import (
    LLMRequest,
    LLMResult,
    Message,
    ToolSpec,
    Usage,
)

# 同一个供应商只建一个实例，复用 HTTP 连接
_PROVIDERS: dict[str, ChatCompletionsProvider] = {}


def get_provider(name: str) -> ChatCompletionsProvider:
    cached = _PROVIDERS.get(name)
    if cached is not None:
        return cached

    config = loader.PROVIDERS.get(name)
    if not config:
        raise LLMError(f"provider '{name}' 未在 llm_relay/config/config.json 中定义")

    provider = ChatCompletionsProvider(name, config)
    _PROVIDERS[name] = provider
    return provider


async def aclose_providers() -> None:
    """关闭所有 provider 的 HTTP 连接（进程退出或测试清理时用）。"""
    for provider in list(_PROVIDERS.values()):
        await provider.aclose()
    _PROVIDERS.clear()


class LLMClient:
    def __init__(self, provider_getter=get_provider):
        self._get_provider = provider_getter

    async def complete(
        self,
        messages: list[Message],
        *,
        task: str = "chat",
        model: str | None = None,
        tools: list[ToolSpec] | None = None,
        tool_choice: str | None = None,
        temperature: float | None = None,
        max_tokens: int | None = None,
    ) -> LLMResult:
        """非流式调用；返回完整 content / tool_calls / usage / cost。"""
        model_key, model_cfg, provider, request = self._resolve(
            messages, task, model, temperature, max_tokens, tools, tool_choice
        )
        attempts = self._attempts()

        for attempt in range(attempts):
            try:
                result = await provider.complete(request)
            except LLMError as e:
                if not e.retryable or attempt == attempts - 1:
                    raise
                await asyncio.sleep(self._backoff(attempt))
                continue

            result.model = model_key
            result.cost_usd = _cost(model_cfg, result.usage)
            return result

        raise LLMError("调用失败：重试次数用尽")

    def _resolve(
        self,
        messages: list[Message],
        task: str,
        model: str | None,
        temperature: float | None,
        max_tokens: int | None,
        tools: list[ToolSpec] | None,
        tool_choice: str | None,
    ):
        model_key = model or loader.ROUTING.get(task) or loader.DEFAULTS.get("model")
        if not model_key:
            raise LLMError(f"任务 '{task}' 没有配置模型，且 defaults.model 为空")

        model_cfg = loader.MODELS.get(model_key)
        if not model_cfg:
            raise LLMError(f"模型 '{model_key}' 未在 config.json 的 models 中定义")

        provider = self._get_provider(model_cfg["provider"])
        request = LLMRequest(
            model=model_cfg.get("api_model", model_key),
            messages=messages,
            temperature=float(
                temperature
                if temperature is not None
                else model_cfg.get("temperature", loader.DEFAULTS.get("temperature", 0.7))
            ),
            max_tokens=int(
                max_tokens
                if max_tokens is not None
                else model_cfg.get("max_tokens", loader.DEFAULTS.get("max_tokens", 2048))
            ),
            tools=tools,
            tool_choice=tool_choice,
        )
        return model_key, model_cfg, provider, request

    def _attempts(self) -> int:
        return max(1, int(loader.HTTP.get("retry_attempts", 3)))

    def _backoff(self, attempt: int) -> float:
        backoff = float(loader.HTTP.get("retry_backoff_seconds", 1.0))
        max_backoff = float(loader.HTTP.get("retry_max_backoff_seconds", 10.0))
        return min(backoff * (2 ** attempt), max_backoff)


def _cost(model_cfg: dict, usage: Usage | None) -> float:
    """按 models.<name> 的价格算美元成本（价格单位：美元 / 百万 token）。"""
    if not usage:
        return 0.0
    input_price = float(model_cfg.get("input_price", 0.0))
    output_price = float(model_cfg.get("output_price", 0.0))
    return (usage.prompt_tokens * input_price + usage.completion_tokens * output_price) / 1_000_000


llm = LLMClient()
