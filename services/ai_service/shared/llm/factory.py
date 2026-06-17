"""LLM provider 注册与工厂。

使用装饰器模式注册 provider：
    @register("my_provider")
    class MyProvider(AbstractLLM):
        ...

    get_llm() 根据 .env 中的 LLM_PROVIDER 查找并实例化。
"""

from config.settings import settings
from shared.llm.base import AbstractLLM

# 全局注册表：provider 名称 → 类
_registry: dict[str, type[AbstractLLM]] = {}


def register(provider_name: str):
    """装饰器工厂：返回一个装饰器，将类注册到工厂。

    用法：
        @register("deepseek")
        class DeepSeekLLM(OpenAICompatibleLLM):
            ...
    """
    def decorator(cls: type[AbstractLLM]) -> type[AbstractLLM]:
        _registry[provider_name] = cls
        return cls
    return decorator


def get_llm(provider: str | None = None) -> AbstractLLM:
    """获取 LLM 实例。

    provider 参数为空时使用 settings.llm_provider（默认从 .env 读取）。
    未找到注册项时抛出 ValueError，包含已注册的 provider 列表方便排查。
    """
    name = provider or settings.llm_provider
    cls = _registry.get(name)
    if cls is None:
        raise ValueError(
            f"未知的 LLM provider '{name}'，"
            f"已注册: {list(_registry.keys())}"
        )
    return cls()


# provider 注册由 main.py 启动时显式 import 触发，避免循环引用
