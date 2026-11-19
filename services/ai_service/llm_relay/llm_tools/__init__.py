"""llm_tools：模型可调用的工具层。

    registry.py   注册表：name -> Tool，唯一真相
    catalog.py    常驻目录：name + description，schema 按需加载
    loader.py     Toolbox / load_tools：查询 schema 并激活
    web/          HTTP 型工具，共用 web/client.py
    local/        本地型工具

新增工具：在 web/ 或 local/ 下新建一个以工具名命名的包，
在包内用 @tool 声明 name / description / parameters / handler，
再在本文件 import 一次触发注册。
"""

from llm_relay.llm_tools.registry import Tool, ToolContext, registry, tool
from llm_relay.llm_tools.loader import Toolbox

# 显式 import 工具包，触发注册（顺序无关）
from llm_relay.llm_tools.local import get_current_time as _get_current_time  # noqa: F401
from llm_relay.llm_tools.web import web_fetch as _web_fetch  # noqa: F401
from llm_relay.llm_tools.web import web_search as _web_search  # noqa: F401

__all__ = ["Tool", "ToolContext", "Toolbox", "registry", "tool"]
