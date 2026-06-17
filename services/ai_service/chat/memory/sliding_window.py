"""滑动窗口对话记忆。

工作原理：
    1. 用 collections.deque 保存最近 N 轮对话（一轮 = user + assistant 各一条）
    2. deque 设置了 maxlen，超出后自动丢弃最旧的条目
    3. build() 方法将 system_prompt + 记忆拼装为 LLM 可用的消息列表

当前阶段（v1）：
    仅靠 deque 容量限制，超出直接丢弃。

未来扩展（v2）：
    超出 summary_threshold 后，把旧消息送给 LLM 做摘要，
    摘要文本注入 system prompt，释放窗口空间给新对话。
"""

from collections import deque

from config.settings import settings
from shared.llm.base import LLMMessage


class SlidingWindowMemory:
    """管理一个用户与 AI 之间的对话记忆。"""

    def __init__(
        self,
        window_size: int | None = None,
        summary_threshold: int | None = None,
    ) -> None:
        # 窗口大小：保留的对话轮数
        self.window_size = window_size or settings.sliding_window_size
        # 摘要阈值：超出此轮数后触发摘要（v2 实现）
        self.summary_threshold = (
            summary_threshold or settings.sliding_window_summary_threshold
        )
        # maxlen = 窗口 × 2，因为每条 user 消息对应一条 assistant 回复
        self._messages: deque[LLMMessage] = deque(maxlen=self.window_size * 2)

    def add(self, role: str, content: str) -> None:
        """向窗口末尾添加一条消息。超出 maxlen 时最旧条目自动弹出。"""
        self._messages.append(LLMMessage(role=role, content=content))

    def build(self, system_prompt: str = "") -> list[LLMMessage]:
        """构造发送给 LLM 的完整消息序列。

        顺序：system_prompt（如有）→ 历史对话（从旧到新）。
        """
        result: list[LLMMessage] = []
        if system_prompt:
            result.append(LLMMessage(role="system", content=system_prompt))
        result.extend(self._messages)
        return result

    def clear(self) -> None:
        """清空所有记忆，适合用户主动重置对话场景。"""
        self._messages.clear()
