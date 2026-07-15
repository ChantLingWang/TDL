"""系统时间工具。"""

from datetime import datetime, timezone, timedelta

CST = timezone(timedelta(hours=8))


def get_current_time() -> str:
    """返回当前北京时间，格式 ISO 8601。"""
    return datetime.now(CST).strftime("%Y-%m-%dT%H:%M:%S+08:00")


def get_current_time_readable() -> str:
    """返回当前北京时间，可读格式。"""
    return datetime.now(CST).strftime("%Y年%m月%d日 %H:%M:%S（北京时间）")
