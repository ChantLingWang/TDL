"""SiliconFlow API keys 和模型配置。"""

from config.settings import settings

EMBEDDING_API_KEY = settings.siliconflow_embedding_api_key
RERANKER_API_KEY = settings.siliconflow_reranker_api_key

EMBEDDING_MODEL = "Qwen/Qwen3-Embedding-8B"
RERANKER_MODEL = "Qwen/Qwen3-Reranker-8B"
EMBEDDING_DIM = 4096

BASE_URL = "https://api.siliconflow.cn/v1"
