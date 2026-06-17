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

    # ---- AI 身份 ----
    ai_user_id: str = "ai-assistant"
    """AI 在 chat_service 中的用户 ID"""

    # ---- Kafka ----
    kafka_brokers: str = "localhost:9094"
    kafka_topic: str = "chat_group_message"
    kafka_group_id: str = "ai_service_group"

    # ---- chat_service ----
    chat_service_url: str = "http://localhost:8080"

    # ---- 成本审计 ----
    cost_tracking_enabled: bool = True
    cost_table_name: str = "llm_api_costs"
    cost_db_host: str = "localhost"
    cost_db_port: int = 5432
    cost_db_user: str = "postgres"
    cost_db_password: str = "postgres"
    cost_db_name: str = "ai_audit"

    # ---- 滑动窗口 ----
    sliding_window_size: int = 20
    sliding_window_summary_threshold: int = 10


settings = Settings()
