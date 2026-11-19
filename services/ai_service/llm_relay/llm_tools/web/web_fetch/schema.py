"""web_fetch 的参数定义。"""

PARAMETERS = {
    "type": "object",
    "properties": {
        "url": {"type": "string", "description": "http/https 网页地址"},
    },
    "required": ["url"],
    "additionalProperties": False,
}
