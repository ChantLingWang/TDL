"""llm_relay 配置包。

只暴露 loader 模块，调用方按需从 loader 上取字段：

    from llm_relay.config import loader

    model = loader.MODELS["deepseek-chat"]
    key = loader.api_key("deepseek")
"""

from . import loader

__all__ = ["loader"]
