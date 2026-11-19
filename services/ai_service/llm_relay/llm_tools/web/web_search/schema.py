"""web_search 的参数定义。"""

PARAMETERS = {
    "type": "object",
    "properties": {
        "query": {"type": "string", "description": "搜索关键词"},
        "count": {
            "type": "integer",
            "minimum": 1,
            "maximum": 50,
            "description": "返回条数，默认 5",
        },
        "need_content": {
            "type": "boolean",
            "description": "是否同时返回网页正文",
        },
    },
    "required": ["query"],
    "additionalProperties": False,
}
