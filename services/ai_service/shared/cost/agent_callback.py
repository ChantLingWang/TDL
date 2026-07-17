"""LangChain callback that accumulates token usage across an agent graph run.

Used by agent/service.py to record total LLM cost after the research graph completes.
"""

from langchain_core.callbacks import BaseCallbackHandler


class CostTrackingCallback(BaseCallbackHandler):
    """Accumulates prompt/completion tokens across all LLM invocations in a graph run."""

    def __init__(self) -> None:
        super().__init__()
        self.prompt_tokens: int = 0
        self.completion_tokens: int = 0
        self.model: str = ""

    def on_llm_end(self, response, **kwargs) -> None:
        usage = getattr(response, "llm_output", {}) or {}
        token_usage = usage.get("token_usage", {})
        self.prompt_tokens += token_usage.get("prompt_tokens", 0)
        self.completion_tokens += token_usage.get("completion_tokens", 0)
        if not self.model:
            self.model = usage.get("model_name", "")

    @property
    def total_tokens(self) -> int:
        return self.prompt_tokens + self.completion_tokens
