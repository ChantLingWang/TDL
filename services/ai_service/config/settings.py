"""全局配置模块。

所有配置项通过 pydantic-settings 从 .env 文件和环境变量读取，
优先级：环境变量 > .env 文件 > 代码默认值。

添加新模型时：
    1. 在此文件新增专属配置段（DEEPSEEK_* / OPENAI_* 等）
    2. 在对应 provider 类的 __init__ 中读取专属配置
"""

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """应用配置。字段名大写映射到环境变量。"""

    model_config = {"env_file": ".env", "env_file_encoding": "utf-8"}

    # ---- 网络代理 ----
    http_proxy: str = ""
    """HTTP 代理地址，如 http://127.0.0.1:7890，为空则不使用代理"""

    # ---- SearXNG ----
    searxng_base_url: str = "http://localhost:8888"
    """SearXNG 服务地址"""

    # ---- 默认 LLM 配置（provider 未指定时的兜底） ----
    llm_provider: str = "deepseek"
    """当前使用的 provider 名称，需在 factory 中已注册"""
    llm_max_tokens: int = 2048
    """单次回复最大 token 数"""
    llm_temperature: float = 0.7
    """生成温度，越高越随机"""

    # ---- DeepSeek ----
    deepseek_api_key: str = ""
    deepseek_base_url: str = "https://api.deepseek.com/v1"
    deepseek_model: str = "deepseek-chat"

    # ---- OpenAI ----
    openai_api_key: str = ""
    openai_base_url: str = "https://api.openai.com/v1"
    openai_model: str = "gpt-4o-mini"

    # ---- SiliconFlow（embedding / rerank）----
    siliconflow_embedding_api_key: str = ""
    siliconflow_reranker_api_key: str = ""

    # ---- AI 身份 ----
    ai_user_id: str = "ai-assistant"
    """AI 在 chat_service 中的用户 ID（chat 模式）"""


    # ---- Qdrant ----
    qdrant_url: str = "http://localhost:6333"
    qdrant_collection: str = "long_term_memory"
    ai_research_user_id: str = "ai-research"
    """AI 研究型用户 ID（agent 模式，走 research 图）"""

    # ---- Kafka ----
    kafka_brokers: str = "localhost:9094"
    kafka_topic: str = "chat_group_message"
    kafka_group_id: str = "ai_service_group"

    # ---- chat_service ----
    chat_service_url: str = "http://localhost:8080"
    internal_api_key: str = ""
    """调用 chat_service 内部接口（历史）时使用的密钥"""

    # ---- 成本审计 ----
    cost_tracking_enabled: bool = True
    cost_table_name: str = "llm_api_costs"
    cost_db_host: str = "localhost"
    cost_db_port: int = 5432
    cost_db_user: str = "postgres"
    cost_db_password: str = "postgres"
    cost_db_name: str = "ai_audit"

    # ---- 短期记忆（DSH 式：真相在历史 + 折叠检查点 + token 预算）----
    # 数值全部对齐 DSH（dsh-compaction-basic / dsh-llm-deepseek）：
    #   DeepSeek V4 上下文窗口 = 1,000,000 token（DEFAULT_CONTEXT_WINDOW = 1e6）
    #   thresholdRatio = 0.8（触发线 = 窗口 × 0.8 = 80 万 token）
    #   retainRatio    = 0.16（保留尾巴 = 窗口 × 0.16 = 16 万 token）
    #   摘要 maxTokens  = 8192
    memory_context_window: int = 1000000
    """模型上下文窗口 token 数（DeepSeek V4 = 1,000,000，对齐 DSH DEFAULT_CONTEXT_WINDOW）"""
    memory_summary_trigger_ratio: float = 0.8
    """触发折叠的窗口占用比例（对齐 DSH thresholdRatio=0.8）"""
    memory_retain_ratio: float = 0.16
    """折叠后保留的原文尾巴比例（对齐 DSH retainRatio=0.16）"""
    memory_summary_max_tokens: int = 8192
    """摘要调用的输出 token 上限（对齐 DSH compaction maxTokens=8192）"""
    memory_mongo_url: str = "mongodb://localhost:27017"
    """折叠检查点存储（与 chat_service 历史同库）"""
    memory_mongo_db: str = "chat"
    memory_checkpoint_collection: str = "conversation_memory"
    """折叠检查点集合：{conversation_id, watermark_ms, summary, updated_at_ms}"""


settings = Settings()
