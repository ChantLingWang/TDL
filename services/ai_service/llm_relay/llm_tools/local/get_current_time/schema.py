"""get_current_time 的参数定义。"""

PARAMETERS = {
    "type": "object",
    "properties": {
        "readable": {
            "type": "boolean",
            "description": "true 返回中文可读格式，默认 false（ISO 8601）",
        }
    },
    "additionalProperties": False,
}
