"""Agent 路由：根据消息内容选择 skill 对应的图。"""


def select_skill(content: str) -> str:
    """根据用户输入关键词选择 skill。

    返回 skill 名称，空字符串表示使用默认图。
    """
    content_lower = content.lower()

    if any(kw in content_lower for kw in ["经济", "gdp", "通胀", "economy"]):
        return "economy"

    # 后续在此扩展：
    # if any(kw in content_lower for kw in ["论文", "文献", "research"]):
    #     return "research"

    return ""   # 默认图
