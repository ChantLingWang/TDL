"""短期记忆（DSH 式）纯逻辑单元测试：token 估算、时间归一化、折叠切分。"""

import asyncio
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from chat.memory.context import (
    estimate_tokens,
    format_history,
    _to_epoch_ms,
)
from shared.models import ChatHistoryMessage


def test_estimate_tokens():
    # 纯中文：1 字 1 token
    assert estimate_tokens("你好世界") == 4
    # 纯英文：4 字符 1 token
    assert estimate_tokens("hello") == 2  # 5 字符 → ceil(5/4) = 2
    # 空串
    assert estimate_tokens("") == 0
    # 混合
    assert estimate_tokens("你好ab") == 2 + 1  # 2 CJK + ceil(2/4)


def test_to_epoch_ms():
    assert _to_epoch_ms(1786000000000) == 1786000000000
    iso = _to_epoch_ms("2026-08-07T06:44:08.073Z")
    assert 1786085048000 - 2000 < iso < 1786085048000 + 2000
    assert _to_epoch_ms(None) == 0
    assert _to_epoch_ms("not-a-date") == 0


def test_format_history():
    msgs = [
        ChatHistoryMessage(sender_id="u1", content="你好", timestamp=1, message_id="m1"),
        ChatHistoryMessage(sender_id="ai-assistant", content="您好", timestamp=2, message_id="m2"),
    ]
    text = format_history(msgs)
    assert "用户: 你好" in text
    assert "AI: 您好" in text


if __name__ == "__main__":
    test_estimate_tokens()
    test_to_epoch_ms()
    test_format_history()
    print("memory unit tests PASS")
