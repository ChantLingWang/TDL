"""DeepSeek provider 完全兼容 OpenAI 接口。

定价（$/百万token）：
    deepseek-chat:     输入 $0.27  输出 $1.10
    deepseek-reasoner: 输入 $0.55  输出 $2.19

配置方式（.env）：
    DEEPSEEK_API_KEY=sk-xxx
    DEEPSEEK_BASE_URL=https://api.deepseek.com/v1
    DEEPSEEK_MODEL=deepseek-chat
"""

from config.settings import settings
from shared.llm.factory import register
from shared.llm.providers.openai_compatible import OpenAICompatibleLLM


@register("deepseek")
class DeepSeekLLM(OpenAICompatibleLLM):
    """DeepSeek，继承 OpenAI 兼容逻辑。"""

    PRICING = {
        "deepseek-chat":     {"input": 0.27, "output": 1.10},
        "deepseek-reasoner": {"input": 0.55, "output": 2.19},
    }

    def __init__(self) -> None:
        # 读取 DeepSeek 专属配置
        self._base = settings.deepseek_base_url.rstrip("/")
        self._model = settings.deepseek_model
        self._key = settings.deepseek_api_key
        self._create_client()
