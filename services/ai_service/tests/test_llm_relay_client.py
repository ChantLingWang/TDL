"""llm_relay.client 单元测试：全部用假 provider，不触网。"""

import asyncio
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from llm_relay.client import LLMClient  # noqa: E402
from llm_relay.config import loader  # noqa: E402
from llm_relay.errors import LLMError  # noqa: E402
from llm_relay.types import (  # noqa: E402
    LLMResult,
    Message,
    ToolCall,
    Usage,
)


class FakeProvider:
    """只实现 complete；按 failures 次数先抛可重试错误。"""

    def __init__(self, failures=0, result=None, error=None):
        self.failures = failures
        self.error = error
        self.result = result or LLMResult(content="ok", usage=Usage(1000, 500, 1500))
        self.calls = 0

    async def complete(self, request):
        self.calls += 1
        if self.calls <= self.failures:
            raise self.error or LLMError("boom", retryable=True)
        return self.result

    async def aclose(self):
        return None


def _complete(client, **kwargs):
    async def run():
        return await client.complete([Message("user", "hi")], **kwargs)

    return asyncio.run(run())


def test_complete_returns_content_and_cost():
    result = _complete(LLMClient(provider_getter=lambda name: FakeProvider()))
    assert result.content == "ok"
    assert result.model == "deepseek-chat"
    assert abs(result.cost_usd - 0.00082) < 1e-9


def test_complete_returns_tool_calls():
    call = ToolCall(id="call_1", name="get_weather", arguments={"city": "北京"})
    fake = FakeProvider(result=LLMResult(content="", tool_calls=[call]))
    result = _complete(LLMClient(provider_getter=lambda name: fake), tools=[])
    assert result.tool_calls[0].name == "get_weather"
    assert result.tool_calls[0].arguments["city"] == "北京"


def test_complete_retries():
    fake = FakeProvider(failures=1)
    old = loader.HTTP["retry_backoff_seconds"]
    loader.HTTP["retry_backoff_seconds"] = 0
    try:
        result = _complete(LLMClient(provider_getter=lambda name: fake))
    finally:
        loader.HTTP["retry_backoff_seconds"] = old

    assert fake.calls == 2
    assert result.content == "ok"


def test_non_retryable_error_not_retried():
    fake = FakeProvider(failures=1, error=LLMError("bad request", status=400, retryable=False))
    try:
        _complete(LLMClient(provider_getter=lambda name: fake))
    except LLMError as e:
        assert e.status == 400
    else:
        raise AssertionError("expected LLMError")

    assert fake.calls == 1


def test_unknown_model_raises():
    client = LLMClient(provider_getter=lambda name: FakeProvider())
    try:
        _complete(client, model="nope")
    except LLMError as e:
        assert "nope" in str(e)
    else:
        raise AssertionError("expected LLMError")
