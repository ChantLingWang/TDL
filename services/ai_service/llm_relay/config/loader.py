"""llm_relay 配置加载。

配置文件默认是 llm_relay/config/config.json，可用环境变量 LLM_RELAY_CONFIG 指向别的路径。
文件里只有一个顶层大块 "llm_relay"，内部按类型分成：

    defaults   默认模型与采样参数
    providers  连接信息（base_url、api_key_env，密钥本身不进 JSON）
    models     模型能力与价格（指向 provider）
    routing    任务名 → 模型名
    http       超时、重试、代理环境变量名
    web        搜索/抓取接口（DeepSeek /tools/search、/tools/fetch）

用法（其他文件导入 loader 模块，按需取字段）：

    from llm_relay.config import loader

    model = loader.MODELS["deepseek-chat"]
    provider = loader.PROVIDERS[model["provider"]]
    key = loader.api_key("deepseek")
"""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any

from config.settings import settings

DEFAULT_CONFIG_PATH = Path(__file__).with_name("config.json")

_BLOCKS = ("defaults", "providers", "models", "routing", "http", "web")


class ConfigError(RuntimeError):
    """llm_relay 配置缺失或非法。"""


def _require(block: dict, key: str, where: str) -> Any:
    if key not in block:
        raise ConfigError(f"{where} 缺少字段 '{key}'")
    return block[key]


def _validate(raw: dict) -> dict:
    """校验结构，返回 "llm_relay" 大块。

    只做结构校验，不校验业务语义（例如价格是否合理）。
    """
    llm_relay = _require(raw, "llm_relay", "根配置")
    if not isinstance(llm_relay, dict):
        raise ConfigError("'llm_relay' 必须是对象")

    for name in _BLOCKS:
        if name not in llm_relay:
            raise ConfigError(f"llm_relay 缺少配置块 '{name}'")
        if not isinstance(llm_relay[name], dict):
            raise ConfigError(f"llm_relay.{name} 必须是对象")

    for name, provider in llm_relay["providers"].items():
        for field in ("base_url", "api_key_env"):
            if field not in provider:
                raise ConfigError(f"providers.{name} 缺少字段 '{field}'")

    for name, model in llm_relay["models"].items():
        provider = model.get("provider")
        if provider not in llm_relay["providers"]:
            raise ConfigError(f"models.{name} 引用了不存在的 provider '{provider}'")

    for task, model in llm_relay["routing"].items():
        if model not in llm_relay["models"]:
            raise ConfigError(f"routing.{task} 引用了不存在的 model '{model}'")

    default_model = llm_relay["defaults"].get("model")
    if default_model and default_model not in llm_relay["models"]:
        raise ConfigError(f"defaults.model 引用了不存在的 model '{default_model}'")

    web = llm_relay["web"]
    if web.get("provider") not in llm_relay["providers"]:
        raise ConfigError(f"web.provider 引用了不存在的 provider '{web.get('provider')}'")
    for key in ("search_path", "fetch_path"):
        if not str(web.get(key, "")).startswith("/"):
            raise ConfigError(f"web.{key} 必须以 / 开头")
    if int(web.get("default_count", 5)) > int(web.get("max_count", 50)):
        raise ConfigError("web.default_count 不能大于 web.max_count")

    return llm_relay


def load(path: str | os.PathLike | None = None) -> dict:
    """读取并校验配置，返回 llm_relay 大块。"""
    p = Path(path or os.getenv("LLM_RELAY_CONFIG") or DEFAULT_CONFIG_PATH).expanduser()
    if not p.is_file():
        raise ConfigError(f"配置文件不存在: {p}")
    try:
        raw = json.loads(p.read_text(encoding="utf-8"))
    except json.JSONDecodeError as e:
        raise ConfigError(f"配置文件不是合法 JSON: {p} ({e})") from e
    return _validate(raw)


def api_key(provider: str) -> str:
    """读取密钥：真实环境变量优先，其次 .env（由 pydantic-settings 载入）。

    providers.<name>.api_key_env 里只存变量名，例如 DEEPSEEK_API_KEY；
    .env 中的同名变量会被 config.settings 读成 deepseek_api_key 字段，
    因此按“变量名小写”回退取一次。JSON 中永远不出现密钥本身。
    """
    try:
        env_name = PROVIDERS[provider]["api_key_env"]
    except KeyError as e:
        raise ConfigError(f"未知 provider: {provider}") from e

    value = os.getenv(env_name, "")
    if value:
        return value
    return str(getattr(settings, env_name.lower(), "") or "")


# 模块导入时加载一次；其他文件直接 from llm_relay.config import XXX
_CONFIG = load()

DEFAULTS: dict = _CONFIG["defaults"]
PROVIDERS: dict = _CONFIG["providers"]
MODELS: dict = _CONFIG["models"]
ROUTING: dict = _CONFIG["routing"]
HTTP: dict = _CONFIG["http"]
WEB: dict = _CONFIG["web"]

__all__ = [
    "DEFAULTS",
    "PROVIDERS",
    "MODELS",
    "ROUTING",
    "HTTP",
    "WEB",
    "ConfigError",
    "load",
    "api_key",
]
