"""Chat Completions 协议驱动。

负责：把 LLMRequest 拼成 /chat/completions 的 JSON，发 HTTP，
解析响应/SSE，并把 HTTP 错误映射成 LLMError。
所有兼容该接口的供应商共用这一个文件。
"""

from __future__ import annotations

import json
import logging
import os

import httpx

from config.settings import settings
from llm_relay.config import loader
from llm_relay.errors import LLMError
from llm_relay.types import (
    LLMRequest,
    LLMResult,
    ToolCall,
    Usage,
)

logger = logging.getLogger(__name__)


class ChatCompletionsProvider:
    def __init__(self, name: str, config: dict):
        self.name = name
        self.config = config
        timeout = float(config.get("timeout_seconds", loader.HTTP.get("timeout_seconds", 60)))
        self._client = httpx.AsyncClient(
            timeout=httpx.Timeout(timeout),
            proxy=_proxy(),
        )

    @property
    def url(self) -> str:
        return self.config["base_url"].rstrip("/") + "/chat/completions"

    def _headers(self) -> dict[str, str]:
        key = loader.api_key(self.name)
        if not key:
            env_name = self.config.get("api_key_env", "?")
            raise LLMError(f"{self.name} 缺少 API Key，请检查 .env 的 {env_name}")
        headers = {
            "Authorization": f"Bearer {key}",
            "Content-Type": "application/json",
        }
        headers.update(self.config.get("headers") or {})
        return headers

    def _messages(self, request: LLMRequest) -> list[dict]:
        messages: list[dict] = []
        for message in request.messages:
            item: dict = {"role": message.role, "content": message.content}
            if message.tool_calls:
                item["tool_calls"] = [
                    {
                        "id": call.id,
                        "type": "function",
                        "function": {
                            "name": call.name,
                            "arguments": call.raw_arguments or json.dumps(
                                call.arguments, ensure_ascii=False
                            ),
                        },
                    }
                    for call in message.tool_calls
                ]
            if message.role == "tool" and message.tool_call_id:
                item["tool_call_id"] = message.tool_call_id
            messages.append(item)
        return messages

    def _payload(self, request: LLMRequest) -> dict:
        payload: dict = {
            "model": request.model,
            "messages": self._messages(request),
            "temperature": request.temperature,
            "max_tokens": request.max_tokens,
        }
        if request.tools:
            payload["tools"] = [
                {
                    "type": "function",
                    "function": {
                        "name": tool.name,
                        "description": tool.description,
                        "parameters": tool.parameters,
                    },
                }
                for tool in request.tools
            ]
            if request.tool_choice:
                payload["tool_choice"] = request.tool_choice
        return payload

    async def complete(self, request: LLMRequest) -> LLMResult:
        """非流式调用；工具轮和最终回答都走这里，能拿到完整 tool_calls。"""
        payload = self._payload(request)
        try:
            resp = await self._client.post(
                self.url, headers=self._headers(), json=payload
            )
        except httpx.HTTPError as e:
            raise LLMError(f"{self.name} 请求失败: {e}", retryable=True) from e

        if resp.status_code >= 400:
            raise _error(resp.status_code, resp.content)

        try:
            body = resp.json()
        except Exception as e:
            raise LLMError(f"{self.name} 返回非 JSON: {e}", retryable=True) from e

        choice = (body.get("choices") or [{}])[0]
        message = choice.get("message") or {}
        return LLMResult(
            content=message.get("content") or "",
            reasoning=message.get("reasoning_content") or "",
            tool_calls=_parse_tool_calls(message.get("tool_calls") or []),
            finish_reason=str(choice.get("finish_reason") or ""),
            usage=_usage(body.get("usage") or {}),
        )

    async def aclose(self) -> None:
        await self._client.aclose()


def _proxy() -> str | None:
    env_name = loader.HTTP.get("proxy_env", "HTTP_PROXY")
    return os.getenv(env_name) or settings.http_proxy or None


def _usage(raw: dict) -> Usage:
    return Usage(
        prompt_tokens=int(raw.get("prompt_tokens", 0) or 0),
        completion_tokens=int(raw.get("completion_tokens", 0) or 0),
        total_tokens=int(raw.get("total_tokens", 0) or 0),
    )


def _parse_tool_calls(raw: list[dict]) -> list[ToolCall]:
    calls: list[ToolCall] = []
    for item in raw:
        function = item.get("function") or {}
        raw_arguments = function.get("arguments") or ""
        arguments: dict = {}
        parse_error = ""
        if raw_arguments:
            try:
                parsed = json.loads(raw_arguments)
                if isinstance(parsed, dict):
                    arguments = parsed
                else:
                    parse_error = "arguments 不是 JSON 对象"
            except json.JSONDecodeError as e:
                parse_error = str(e)
        calls.append(
            ToolCall(
                id=item.get("id") or "",
                name=function.get("name") or "",
                arguments=arguments,
                raw_arguments=raw_arguments,
                parse_error=parse_error,
            )
        )
    return calls


def _error(status: int, body: bytes) -> LLMError:
    text = body.decode("utf-8", "replace")
    message = text[:300]
    try:
        message = (json.loads(text).get("error") or {}).get("message") or message
    except Exception:
        pass
    retryable = status in (408, 409, 429) or status >= 500
    return LLMError(message, status=status, retryable=retryable)
