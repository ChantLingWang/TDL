"""SiliconFlow API keys 和模型配置。"""

import os

EMBEDDING_API_KEY = os.getenv(
    "SILICONFLOW_EMBEDDING_API_KEY",
    "sk-tyusvruhvrgddrvqmtbvahsmmuikzmseeucmarevfqrrnleh",
)
RERANKER_API_KEY = os.getenv(
    "SILICONFLOW_RERANKER_API_KEY",
    "sk-ffoqjgptuqpxrnkelaciqfslbmmbfujohyclyqdoobclrbty",
)

EMBEDDING_MODEL = "Qwen/Qwen3-Embedding-8B"
RERANKER_MODEL = "Qwen/Qwen3-Reranker-8B"
EMBEDDING_DIM = 4096

BASE_URL = "https://api.siliconflow.cn/v1"
