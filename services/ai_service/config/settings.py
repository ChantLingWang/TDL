"""全局配置模块。

只放两类东西：
    1. 基础设施连接（Kafka / 成本库 / 记忆检查点）
    2. 密钥（只从 .env 或环境变量读，JSON 里只存变量名）

LLM 的 provider 连接、模型、价格、路由、超时重试全部在
llm_relay/config/config.json，不放这里。

优先级：环境变量 > .env 文件 > 代码默认值。
"""

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """应用配置。字段名大写映射到环境变量。"""

    model_config = {
        "env_file": ".env",
        "env_file_encoding": "utf-8",
        # .env 里残留的旧变量（llm_provider / deepseek_model 等）直接忽略
        "extra": "ignore",
    }

    # ---- 网络代理（工具抓网页用；LLM 走 llm_relay 的 http.proxy_env）----
    http_proxy: str = ""

    # ---- 密钥：只从 .env / 环境变量读取，不写默认值，也不进 JSON ----
    # 新增 provider 时，这里要加一行与 api_key_env 同名的小写字段，
    # 否则 .env 里的 key 读不到（详见 HOW_TO_ADD_MODEL.md）。
    deepseek_api_key: str = ""

    # ---- AI 身份 ----
    ai_user_id: str = "ai-assistant"
    """AI 在 gateway 中的用户 ID；所有会话都由同一个 harness 处理"""

    # ---- Kafka ----
    kafka_brokers: str = "localhost:9094"
    kafka_topic: str = "chat_group_message"
    kafka_group_id: str = "ai_service_group"

    # ---- 成本审计 ----
    cost_tracking_enabled: bool = True
    cost_table_name: str = "llm_api_costs"
    cost_db_host: str = "localhost"
    cost_db_port: int = 5432
    cost_db_user: str = "postgres"
    cost_db_password: str = "postgres"
    cost_db_name: str = "ai_audit"

    # ---- 短期记忆（DSH 式：真相在历史 + 折叠检查点 + token 预算）----
    # 上下文窗口不在这里配，取 llm_relay 默认模型的 context_window。
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
