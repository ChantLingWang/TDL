"""OpenAI 兼容接口 provider。

适用于所有兼容 /v1/chat/completions 的 API：
    - OpenAI
    - DeepSeek
    - 本地 vLLM / Ollama 等

重试策略：
    仅在 httpx.HTTPStatusError 时重试（网络超时/5xx），
    最多 3 次，指数退避 1s / 2s / 4s。
"""

import json
from collections.abc import AsyncIterator

import httpx
from tenacity import (
    retry,
    retry_if_exception_type,
    stop_after_attempt,
    wait_exponential,
)

from config.settings import settings
from shared.llm.base import AbstractLLM, LLMMessage, LLMResponse
from shared.llm.factory import register


@register("openai_compatible")
class OpenAICompatibleLLM(AbstractLLM):
    """OpenAI 兼容接口 provider。

    chat()       —— 非流式，一次返回完整回复
    chat_stream()—— 流式 SSE，逐 token 产出（v1 预留）
    """

    def __init__(self) -> None:
        # 读取 OpenAI 专属配置，未配置时回退到通用 LLM_ 配置
        self._base = (settings.openai_base_url or "").rstrip("/")
        self._model = settings.openai_model
        self._key = settings.openai_api_key
        self._create_client()

    def _create_client(self) -> None:
        """创建 HTTP 客户端，60 秒超时覆盖长回复场景。"""
        self._client = httpx.AsyncClient(timeout=httpx.Timeout(60.0))

    def _headers(self) -> dict[str, str]:
        """构造 Authorization Bearer + Content-Type 请求头。"""
        return {
            "Authorization": f"Bearer {self._key}",
            "Content-Type": "application/json",
        }

    def _to_openai_messages(self, messages: list[LLMMessage]) -> list[dict]:
        """将内部 LLMMessage 列表转换为 OpenAI API 格式。"""
        return [{"role": m.role, "content": m.content} for m in messages]

    @retry(
        retry=retry_if_exception_type(httpx.HTTPStatusError),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
    )
    async def chat(
        self, messages: list[LLMMessage], **kwargs
    ) -> LLMResponse:
        """非流式对话请求。

        失败的 HTTP 状态码（5xx / 429 等）会自动重试最多 3 次。
        """
        # 允许调用方覆盖默认参数
        max_tokens = kwargs.get("max_tokens", settings.llm_max_tokens)
        temperature = kwargs.get("temperature", settings.llm_temperature)

        payload = {
            "model": self._model,
            "messages": self._to_openai_messages(messages),
            "max_tokens": max_tokens,
            "temperature": temperature,
        }

        resp = await self._client.post(
            f"{self._base}/chat/completions",
            headers=self._headers(),
            json=payload,
        )
        resp.raise_for_status()
        body = resp.json()

        # 取第一个 choice 的 message.content
        choice = body["choices"][0]
        return LLMResponse(
            content=choice["message"]["content"],
            model=body.get("model", self._model),
            usage=body.get("usage", {}),
        )

    async def chat_stream(
        self, messages: list[LLMMessage], **kwargs
    ) -> AsyncIterator[str]:
        """流式输出 — 通过 SSE 逐 token 产出。

        OpenAI 流式协议：每行 data: {"choices":[{"delta":{"content":"xxx"}}]}
        遇到 data: [DONE] 表示结束。
        v1 暂未在生产中使用，预留给后续 SSE 升级。
        """
        max_tokens = kwargs.get("max_tokens", settings.llm_max_tokens)
        temperature = kwargs.get("temperature", settings.llm_temperature)

        payload = {
            "model": self._model,
            "messages": self._to_openai_messages(messages),
            "max_tokens": max_tokens,
            "temperature": temperature,
            "stream": True,
        }

        async with self._client.stream(
            "POST",
            f"{self._base}/chat/completions",
            headers=self._headers(),
            json=payload,
        ) as resp:
            resp.raise_for_status()
            async for line in resp.aiter_lines():
                if line.startswith("data: "):
                    data = line[6:]          # 去掉 "data: " 前缀
                    if data == "[DONE]":
                        return
                    try:
                        chunk = json.loads(data)
                        delta = chunk["choices"][0].get("delta", {})
                        if "content" in delta:
                            yield delta["content"]
                    except (json.JSONDecodeError, KeyError, IndexError):
                        # 个别 chunk 解析失败不影响后续
                        continue

    async def close(self) -> None:
        """关闭底层 HTTP 连接。"""
        await self._client.aclose()
