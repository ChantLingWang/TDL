"""llm_tools：注册表、目录、load_tools 测试（使用真实内置工具，不 mock）。"""

import asyncio
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from llm_relay.llm_tools import Toolbox, ToolContext, registry  # noqa: E402
from llm_relay.llm_tools import catalog as catalog_module  # noqa: E402
from llm_relay.llm_tools.registry import Tool, ToolRegistry  # noqa: E402


def test_builtin_tools_registered():
    assert registry.get("load_tools") is not None
    assert registry.get("load_tools").resident is True
    assert registry.get("get_current_time").kind == "local"
    assert registry.get("web_search").kind == "web"
    assert registry.get("web_fetch").kind == "web"


def test_duplicate_name_rejected():
    reg = ToolRegistry()

    async def handler(ctx):
        return "ok"

    reg.register(Tool(name="demo", description="示例", handler=handler))
    try:
        reg.register(Tool(name="demo", description="重复", handler=handler))
    except ValueError as exc:
        assert "重名" in str(exc)
    else:
        raise AssertionError("重名工具必须被拒绝")


def test_catalog_describe_and_exclude():
    reg = ToolRegistry()

    async def handler(ctx):
        return "ok"

    reg.register(Tool(name="a", description="工具 A", handler=handler))
    reg.register(Tool(name="b", description="工具 B", handler=handler))

    assert catalog_module.describe(reg) == {"a": "工具 A", "b": "工具 B"}
    text = catalog_module.render(reg, exclude={"a"})
    assert "- b：工具 B" in text
    assert "- a：" not in text


def test_toolbox_only_exposes_resident_before_load():
    box = Toolbox(ctx=ToolContext(user_id="u1"))
    assert [item.name for item in box.specs()] == ["load_tools"]


def test_load_tools_returns_schema_and_activates():
    box = Toolbox()
    data = json.loads(asyncio.run(box.call("load_tools", {"names": ["web_fetch"]})))

    assert [item["name"] for item in data["loaded"]] == ["web_fetch"]
    assert data["loaded"][0]["parameters"]["properties"]["url"]["type"] == "string"
    assert "web_fetch" in box.loaded
    assert "web_fetch" in [item.name for item in box.specs()]
    assert "web_fetch：" not in box.catalog()

    again = json.loads(asyncio.run(box.call("load_tools", {"names": ["web_fetch"]})))
    assert again["already"] == ["web_fetch"]


def test_load_tools_unknown_and_resident():
    box = Toolbox()
    data = json.loads(asyncio.run(box.call("load_tools", {"names": ["nope", "load_tools"]})))
    assert data["unknown"] == ["nope"]
    assert data["already"] == ["load_tools"]


def test_call_local_tool():
    box = Toolbox()
    box.activate(["get_current_time"])

    assert asyncio.run(box.call("get_current_time", {})).endswith("+08:00")
    assert "北京时间" in asyncio.run(box.call("get_current_time", {"readable": True}))


def test_call_unloaded_tool_is_rejected():
    box = Toolbox()
    data = json.loads(asyncio.run(box.call("web_search", {"query": "x"})))
    assert "尚未加载" in data["error"]


def test_call_unknown_tool_is_rejected():
    box = Toolbox()
    data = json.loads(asyncio.run(box.call("nope", {})))
    assert "未知工具" in data["error"]


def test_call_serializes_dict_and_runs_sync_handler():
    reg = ToolRegistry()

    def handler(ctx, x):
        return {"x": x, "user": ctx.user_id}

    reg.register(Tool(name="echo", description="回显", handler=handler))
    box = Toolbox(registry=reg, ctx=ToolContext(user_id="u1"))
    box.activate(["echo"])

    assert json.loads(asyncio.run(box.call("echo", {"x": 1}))) == {"x": 1, "user": "u1"}
