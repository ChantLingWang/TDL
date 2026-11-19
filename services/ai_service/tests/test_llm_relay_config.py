"""llm_relay 配置加载单元测试。"""

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from llm_relay.config import loader  # noqa: E402


def test_blocks_loaded():
    assert loader.DEFAULTS["model"] in loader.MODELS
    assert loader.HTTP["timeout_seconds"] > 0
    assert loader.HTTP["retry_attempts"] >= 1


def test_web_block():
    assert loader.WEB["provider"] in loader.PROVIDERS
    assert loader.WEB["search_path"].startswith("/")
    assert loader.WEB["fetch_path"].startswith("/")
    assert loader.WEB["default_count"] <= loader.WEB["max_count"]


def test_references_are_valid():
    for model in loader.MODELS.values():
        assert model["provider"] in loader.PROVIDERS
    for model_name in loader.ROUTING.values():
        assert model_name in loader.MODELS


def test_missing_file_raises():
    try:
        loader.load("/nonexistent/llm_relay.json")
    except loader.ConfigError:
        return
    raise AssertionError("expected ConfigError")
