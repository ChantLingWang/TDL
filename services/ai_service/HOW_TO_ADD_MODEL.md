# 如何新增 LLM 模型

> 重构后模型接入统一走 `llm_relay`（`services/ai_service/llm_relay/`）。
> 连接、模型、价格、任务路由全部在 `llm_relay/config/config.json`，
> 业务代码（`apps/`、`runtime/`、`memory/`）不需要改。

## 配置文件结构

`config.json` 顶层只有一个 `llm_relay` 块，内部固定六段：

| 段 | 作用 |
| --- | --- |
| `defaults` | 默认模型、temperature、max_tokens |
| `providers` | 供应商连接：`base_url` + `api_key_env`（只存变量名，不存密钥） |
| `models` | 模型能力与价格：provider、context_window、input/output price |
| `routing` | 任务名 → 模型名（`chat` / `summarize` 等） |
| `http` | 超时、重试、退避、代理环境变量名 |
| `web` | 联网工具的搜索/抓取端点（`web_search` / `web_fetch`） |

---

## 1. 接入一个 OpenAI 兼容供应商（最快路径）

只要对方提供 `/v1/chat/completions` 且响应结构一致，全程不需要写 Python。

### 1.1 在 `providers` 里加连接

```json
"foo": {
  "base_url": "https://api.foo.com/v1",
  "api_key_env": "FOO_API_KEY"
}
```

`api_key_env` 填**环境变量名**，真实的 key 不写进 JSON。

### 1.2 放密钥（二选一）

- **部署环境变量**：`FOO_API_KEY=sk-xxx`（`os.getenv` 直接命中）；
- **本地 `.env`**：`FOO_API_KEY=sk-xxx`，同时在
  [config/settings.py](config/settings.py) 增加字段 `foo_api_key: str = ""`。

原因是 `loader.api_key()` 先读真实环境变量，读不到再回退到
pydantic Settings 的同名小写字段；`.env` 里的值不会自动进 `os.environ`。

### 1.3 在 `models` 里声明模型

```json
"foo-chat": {
  "provider": "foo",
  "api_model": "foo-chat-2026",
  "context_window": 200000,
  "input_price": 0.27,
  "output_price": 1.10
}
```

- `api_model`：供应商真实模型名；省略时直接用 key（`foo-chat`）；
- `context_window`：短期记忆折叠预算要用，**必填**；
- `input_price` / `output_price`：美元 / 百万 token；
- 可选 `temperature`、`max_tokens`，覆盖 `defaults`。

### 1.4 配置路由

```json
"routing": {
  "chat": "foo-chat",
  "summarize": "deepseek-chat"
}
```

`routing.<task>` 决定 `llm.complete(..., task="chat")` 用哪个模型；
`defaults.model` 是没配路由时的兜底。两者都必须指向已存在的 model。

---

## 2. 非 OpenAI 兼容协议

在 `llm_relay/providers/` 新建一个类，实现：

```python
async def complete(self, request: LLMRequest) -> LLMResult: ...
async def aclose(self) -> None: ...
```

然后在 `LLMClient.get_provider()` 的构造分支里按 `provider` 名接入。
目前 `models.<name>.provider` 只支持 `ChatCompletionsProvider`。

---

## 3. 联网工具端点

`web.enabled` 控制 `web_search` / `web_fetch` 是否可用；
`web.provider` 必须是 `providers` 里已有的名字，`search_path` / `fetch_path`
必须以 `/` 开头，`default_count` 不能大于 `max_count`。改完同样重启生效。

---

## 4. 校验与生效

- `llm_relay/config/loader.py` 在 import 时加载并校验一次，**改完要重启进程**；
- 启动即报 `ConfigError` 的常见原因：provider 不存在、routing 指向不存在的 model、
  `web.search_path` 不以 `/` 开头；
- 可用 `LLM_RELAY_CONFIG=/path/to/config.json` 指向另一份配置（测试/多环境）。

## 5. 注意

- 价格只影响成本表里的估价，不参与计费；
- 生产调用链只有 `complete`；provider 不需要实现流式接口；
- 密钥永远只放 `.env` / 环境变量，`config.json` 里只写变量名。
