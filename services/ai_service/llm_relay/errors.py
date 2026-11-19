"""llm_relay 统一异常。"""


class LLMError(RuntimeError):
    """LLM 调用失败。

    status      HTTP 状态码（网络错误时为 None）
    retryable   是否值得重试（408/409/429/5xx、连接错误）
    """

    def __init__(self, message: str, *, status: int | None = None, retryable: bool = False):
        super().__init__(message)
        self.status = status
        self.retryable = retryable
