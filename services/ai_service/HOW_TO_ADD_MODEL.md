# 如何新增 LLM 模型

三步完成，不需要改动 chat/agent/kafka 任何代码。

---

## 1. 创建 provider 文件

在 `shared/llm/providers/` 下新建 `foo.py`：

```python
from config.settings import settings
from shared.llm.factory import register
from shared.llm.base import AbstractLLM, LLMMessage, LLMResponse


@register("foo")          # 名称要和 .env 中 LLM_PROVIDER 一致
class FooLLM(AbstractLLM):

    PRICING = {
        "foo-model": {"input": 0.50, "output": 1.50},
    }

    def __init__(self) -> None:
        self._base = settings.foo_base_url.rstrip("/")
        self._model = settings.foo_model
        self._key = settings.foo_api_key
        # ... 初始化 HTTP 客户端等

    async def chat(self, messages, **kwargs) -> LLMResponse:
        # 实现非流式对话
        ...

    async def chat_stream(self, messages, **kwargs):
        # 实现流式对话（可选）
        ...
```

> 如果新模型兼容 OpenAI 的 `/v1/chat/completions` 接口，直接继承 `OpenAICompatibleLLM` 即可，
> 只需覆盖 `PRICING` 和 `__init__`，参考 `deepseek.py`。

---

## 2. 注册 provider

在 `shared/llm/factory.py` 末尾加一行 import，触发 `@register` 装饰器：

```python
import shared.llm.providers.foo  # noqa: E402, F401
```

---

## 3. 配置 .env 和 settings.py

**`.env`** —— 新增一个配置段：

```bash
# ============================================================
#  Foo
# ============================================================
FOO_API_KEY=sk-xxx
FOO_BASE_URL=https://api.foo.com/v1
FOO_MODEL=foo-model
```

**`config/settings.py`** —— 新增三个字段：

```python
# ---- Foo ----
foo_api_key: str = ""
foo_base_url: str = "https://api.foo.com/v1"
foo_model: str = "foo-model"
```

---

## 切换模型

改 `.env` 一行：

```bash
LLM_PROVIDER=foo
```

定价（`PRICING`）在 provider 类中维护，修改后重新部署即可，历史数据不受影响。

---

## 现有模型参考

| Provider | 文件 | 接口类型 |
|----------|------|---------|
| `deepseek` | `providers/deepseek.py` | OpenAI 兼容 |
| `openai_compatible` | `providers/openai_compatible.py` | OpenAI 兼容 |
